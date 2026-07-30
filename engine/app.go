// Package engine hosts the runtime: window/GL setup, the frame loop, the model
// list and the systems that act on it. A game is expected to build an App with
// New, optionally drive it through the exported API, and call Run.
package engine

import (
	"3d-engine/assets"
	"3d-engine/camera"
	"3d-engine/input"
	"3d-engine/object"
	"3d-engine/shaders"
	"3d-engine/textures"
	"3d-engine/utils"
	"fmt"
	"math"
	"os"
	"os/signal"
	"sort"
	"strconv"
	"syscall"
	"time"

	"github.com/go-gl/gl/v4.6-core/gl"
	"github.com/go-gl/glfw/v3.3/glfw"
	"github.com/go-gl/mathgl/mgl32"
	"google.golang.org/grpc"
)

const fixedUpdateRate = 50

// Options configures an App. The zero value is usable: every field falls back
// to the default the engine used when it was hardcoded in main.
type Options struct {
	ConfigPath string
	ScenePath  string
	Title      string
	SkyboxPath string
	RPCAddr    string
	DisableRPC bool

	// RegisterComponents adds the game's component types to the registry.
	//
	// It exists because New loads the initial scene before it returns, so
	// registering from the caller afterwards is too late: a scene naming a game
	// component would already have failed with "unknown component type". This
	// hook runs after the built-in components and before that load, which is the
	// only window where a game can get its types in.
	RegisterComponents func(r *ComponentRegistry) error
}

func (o *Options) applyDefaults() {
	if o.ConfigPath == "" {
		o.ConfigPath = "./config.yml"
	}
	if o.ScenePath == "" {
		o.ScenePath = "./scene.yml"
	}
	if o.Title == "" {
		o.Title = "3D-Engine"
	}
	if o.SkyboxPath == "" {
		o.SkyboxPath = "./testObjects/skybox"
	}
}

// rpcAddress prefers the explicit Option, falling back to the config file.
func (a *App) rpcAddress() string {
	if a.opts.RPCAddr != "" {
		return a.opts.RPCAddr
	}
	return a.Config.RPC.Address
}

// rpcDisabled is true if either the caller or the config turned the editor
// server off.
func (a *App) rpcDisabled() bool {
	return a.opts.DisableRPC || a.Config.RPC.Disable
}

func toVec3Slice(values [][3]float32) []mgl32.Vec3 {
	out := make([]mgl32.Vec3, 0, len(values))
	for _, value := range values {
		out = append(out, mgl32.Vec3(value))
	}
	return out
}

// App owns every piece of runtime state that used to live as a package-level
// var in main. All GL work happens on the goroutine that calls Run.
type App struct {
	opts   Options
	Config *utils.Config

	Window *glfw.Window
	Camera *camera.Camera
	Scenes *SceneManager

	// Input maps keys to named actions. Rebind through config or at runtime;
	// nothing downstream knows which key an action came from.
	Input *input.Map

	// Components maps scene-file type names to Go constructors. A game
	// registers its behaviours here before calling Run.
	Components *ComponentRegistry

	// State holds the runtime toggles (wireframe, gravity, debug boxes, ...).
	State State

	width  int
	height int

	deltaTime        float32
	lastFrame        float32
	lastFrameCounter float32
	nbFrames         int

	// World holds the scene's entities and owns their locking.
	World *World

	// Assets keeps imported models resident and shared, and is what frees them
	// when the last scene using them goes away.
	Assets *assets.Cache

	// commands carries work from other goroutines back onto the frame loop.
	commands commandQueue

	lightingShader *shaders.Shader
	debugBoxShader *shaders.Shader
	debugRenderer  *debugBoxRenderer
	skybox         *object.Skybox

	physicsDeltaTime float32
	gravityStrength  float32
	gravityDirection mgl32.Vec3
	// gravityAxes are the directions the H key cycles through, from config.
	gravityAxes []mgl32.Vec3

	lastGravityAxisToggle float64

	playerVelocity     mgl32.Vec3
	playerHalfExtents  mgl32.Vec3
	playerCenterOffset mgl32.Vec3
	playerJumpSpeed    float32
	playerGrounded     bool
	lastJumpTime       float64

	collisionDebugDistance float32

	// overlay draws the editor UI on top of the finished frame, if one is set.
	overlay Overlay

	rpc             *grpc.Server
	glfwInitialized bool
}

// New loads the config, opens the window, uploads the initial scene and starts
// the RPC server. It must be called from the goroutine locked to the main OS
// thread. The caller owns the returned App and must Close it.
func New(opts Options) (*App, error) {
	opts.applyDefaults()

	config, err := utils.LoadConfig(opts.ConfigPath)
	if err != nil {
		return nil, fmt.Errorf("could not load config: %w", err)
	}

	a := &App{
		opts:   opts,
		Config: config,
		width:  config.Width,
		height: config.Height,
		Input:  input.NewMap(),
		World:  NewWorld(),

		Components: NewComponentRegistry(),
		Assets:     assets.NewCache(),

		State: State{
			CaptureCursor:  true,
			GravityEnabled: true,
		},

		physicsDeltaTime: 1.0 / float32(fixedUpdateRate),
		gravityStrength:  config.Physics.Gravity,
		gravityAxes:      toVec3Slice(config.Physics.GravityAxes),

		playerHalfExtents:  mgl32.Vec3(config.Player.HalfExtents),
		playerCenterOffset: mgl32.Vec3(config.Player.CenterOffset),
		playerJumpSpeed:    config.Player.JumpSpeed,

		collisionDebugDistance: config.Physics.CollisionDebugDistance,
	}
	a.gravityDirection = a.gravityAxes[0]
	registerBuiltinComponents(a.Components)

	// Before the initial scene load below, so that scene can name the game's
	// components.
	if opts.RegisterComponents != nil {
		if err := opts.RegisterComponents(a.Components); err != nil {
			a.Close()
			return nil, fmt.Errorf("could not register components: %w", err)
		}
	}

	a.Scenes = NewSceneManager(a, config, opts.ScenePath)
	// Give despawned entities a chance to run OnDestroy.
	a.World.onDespawn = a.destroyComponents

	if err := a.initWindow(); err != nil {
		a.Close()
		return nil, err
	}
	if err := a.initResources(); err != nil {
		a.Close()
		return nil, err
	}

	a.Camera = camera.NewCamera(config)
	if err := a.Scenes.LoadScene(a.Scenes.ResolveInitialScenePath()); err != nil {
		a.Close()
		return nil, fmt.Errorf("could not load scene: %w", err)
	}
	a.resetDynamicState()

	// Wrapped rather than passed straight through, for two reasons.
	//
	// State.CaptureCursor is the authority on whether the mouse is flying the
	// camera at all: with the cursor released the pointer belongs to the editor,
	// and moving it towards a panel must not drag the camera along on the way.
	// The overlay check alone is not enough, because an overlay only claims the
	// pointer once it is already over one of its windows — every pixel of the 3D
	// view in between would still have turned the camera.
	a.Window.SetCursorPosCallback(func(w *glfw.Window, xpos, ypos float64) {
		if !a.State.CaptureCursor || a.overlayCapturesMouse() {
			return
		}
		a.Camera.MouseCallback(w, xpos, ypos)
	})
	a.Window.SetScrollCallback(func(w *glfw.Window, xoff, yoff float64) {
		if a.overlayCapturesMouse() {
			return
		}
		a.Camera.ScrollCallback(w, xoff, yoff)
	})
	defaultBindings(a.Input)
	if err := applyConfigBindings(a.Input, config.Input.Actions); err != nil {
		a.Close()
		return nil, fmt.Errorf("could not apply input bindings: %w", err)
	}

	if !a.rpcDisabled() {
		if err := a.startRPCServer(a.rpcAddress()); err != nil {
			a.Close()
			return nil, err
		}
	}

	return a, nil
}

func (a *App) initWindow() error {
	if err := glfw.Init(); err != nil {
		return fmt.Errorf("could not init glfw: %w", err)
	}
	a.glfwInitialized = true

	glfw.WindowHint(glfw.ContextVersionMajor, 4)
	glfw.WindowHint(glfw.ContextVersionMinor, 6)
	glfw.WindowHint(glfw.OpenGLProfile, glfw.OpenGLCoreProfile)

	window, err := glfw.CreateWindow(a.width, a.height, a.opts.Title, nil, nil)
	if err != nil {
		return fmt.Errorf("could not create a window: %w", err)
	}
	a.Window = window

	window.MakeContextCurrent()

	if err := gl.Init(); err != nil {
		return fmt.Errorf("failed to initialize OpenGL: %w", err)
	}

	gl.Viewport(0, 0, int32(a.width), int32(a.height))

	gl.Enable(gl.DEPTH_TEST)
	gl.Enable(gl.CULL_FACE)

	gl.CullFace(gl.BACK)
	gl.FrontFace(gl.CCW)

	glfw.SwapInterval(a.Config.GetVsync())

	window.SetFramebufferSizeCallback(a.onFramebufferSize)
	// Matches State.CaptureCursor, which New defaults to true.
	window.SetInputMode(glfw.CursorMode, glfw.CursorDisabled)

	return nil
}

func (a *App) initResources() error {
	var err error

	a.lightingShader, err = shaders.CreateShaderProgram("lighting.vert", "lighting.frag")
	if err != nil {
		return fmt.Errorf("could not create cube shader: %w", err)
	}

	a.debugBoxShader, err = shaders.CreateShaderProgram("debug_box.vert", "debug_box.frag")
	if err != nil {
		return fmt.Errorf("could not create debug shader: %w", err)
	}
	a.debugRenderer = newDebugBoxRenderer()

	transparent := false
	a.lightingShader.NoTexture, err = textures.Load("./textures/missing.png", &transparent)
	if err != nil {
		return fmt.Errorf("could not load missing texture: %w", err)
	}

	a.skybox = object.CreateSkybox(a.opts.SkyboxPath)
	a.skybox.LoadCubemap()
	a.skybox.Shader, err = shaders.CreateShaderProgram("skybox.vert", "skybox.frag")
	if err != nil {
		return fmt.Errorf("could not create skybox shader: %w", err)
	}
	a.skybox.Shader.SetInt("skybox", int32(a.skybox.SkyboxTextureUnit))

	return nil
}

// Quit asks the frame loop to stop after the current frame. Safe from any
// goroutine.
func (a *App) Quit() {
	if a.Window != nil {
		a.Window.SetShouldClose(true)
	}
}

// Run drives the frame loop until the window is closed.
func (a *App) Run() error {
	fixedDeltaTime := time.Second / time.Duration(fixedUpdateRate)
	ticker := time.NewTicker(fixedDeltaTime)
	defer ticker.Stop()

	// Without this, Ctrl+C kills the process mid-frame and Close never runs, so
	// the shutdown path that releases assets is skipped entirely.
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(signals)

	go func() {
		if _, ok := <-signals; ok {
			utils.Logger().Println("Shutting down")
			a.Quit()
		}
	}()

	for !a.Window.ShouldClose() {
		currentFrame := float32(glfw.GetTime())
		a.deltaTime = currentFrame - a.lastFrame
		a.lastFrame = currentFrame

		utils.Logger().Verbosef("Frame time: %.2f ms\n", a.deltaTime*1000)

		a.fpsCounter()

		a.processInput()

		gl.ClearColor(0.0, 0.0, 0.0, 1.0)
		gl.Clear(gl.COLOR_BUFFER_BIT | gl.DEPTH_BUFFER_BIT)

		// Anything that needs the GL thread — scene loads today, spawns and
		// asset loads later — runs here.
		a.drainCommands()

		select {
		case <-ticker.C:
			a.fixedUpdate()
		default:
		}

		a.startAndUpdateComponents()

		a.render()

		if a.overlay != nil {
			a.overlay.Frame(a)
		}

		a.Window.SwapBuffers()
		glfw.PollEvents()
	}

	return nil
}

// Close releases the GL resources, the window and the RPC listener. It is safe
// to call on a partially constructed App.
func (a *App) Close() {
	// Release anyone blocked in Do before the loop stops draining.
	a.commands.close()

	if a.overlay != nil {
		a.overlay.Close()
		a.overlay = nil
	}

	// Drop the live scene so shutdown frees what it allocated. Clearing the
	// world first runs OnDestroy while the models are still valid; releasing
	// them beforehand would hand destructors freed GL objects.
	if a.World != nil && a.Assets != nil {
		outgoing := a.currentSceneEntities()
		a.World.Replace(nil)
		a.releaseModels(outgoing)
	}

	if a.rpc != nil {
		a.rpc.Stop()
		a.rpc = nil
	}
	if a.debugRenderer != nil {
		a.debugRenderer.Delete()
		a.debugRenderer = nil
	}
	for _, shader := range []**shaders.Shader{&a.lightingShader, &a.debugBoxShader} {
		if *shader != nil {
			(*shader).Delete()
			*shader = nil
		}
	}
	if a.skybox != nil && a.skybox.Shader != nil {
		a.skybox.Shader.Delete()
		a.skybox.Shader = nil
	}
	if a.Window != nil {
		a.Window.Destroy()
		a.Window = nil
	}
	if a.glfwInitialized {
		glfw.Terminate()
		a.glfwInitialized = false
	}
}

// Models runs fn under the read lock. The slice is only valid for the duration
// of the call — do not retain it.
// currentSceneEntities snapshots the live entities so the scene manager can
// release their assets after the swap.
func (a *App) currentSceneEntities() []*Entity {
	var snapshot []*Entity
	a.World.Read(func(entities []*Entity) {
		snapshot = append(snapshot, entities...)
	})
	return snapshot
}

type renderItem struct {
	mesh     *object.Mesh
	modelMat mgl32.Mat4
	distance float32
}

func (a *App) render() {
	shader := a.lightingShader

	// Lighting
	shader.Use()
	shader.SetVec3Val("viewPos", a.Camera.CameraPos)

	projection := a.Camera.ComputeProjection(a.width, a.height)
	shader.SetMat4("projection", projection)
	view := a.Camera.ComputeView()
	shader.SetMat4("view", view)

	shader.SetInt("skybox", int32(a.skybox.SkyboxTextureUnit))

	opaqueItems := make([]renderItem, 0)
	transparentItems := make([]renderItem, 0)
	debugBoxes := make([]debugBox, 0)
	lights := lightSet{}

	a.World.Read(func(entities []*Entity) {
		for _, entity := range entities {
			lights.collect(entity)

			if entity.Renderer == nil || entity.Renderer.Model == nil {
				continue
			}
			model := entity.Renderer.Model
			modelMat := entity.WorldMatrix()

			if a.State.CollisionDebug {
				a.appendDebugBox(&debugBoxes, entity.WorldAABB(), mgl32.Vec3{1.0, 0.2, 0.2})
			}

			for i := range model.Meshes {
				mesh := &model.Meshes[i]
				if a.State.CollisionDebug {
					a.appendDebugBox(&debugBoxes, mesh.WorldAABB(modelMat), mgl32.Vec3{1.0, 0.8, 0.2})
				}

				if mesh.IsTransparent() {
					dist := a.Camera.CameraPos.Sub(mesh.WorldCenter(modelMat)).LenSqr()
					transparentItems = append(transparentItems, renderItem{
						mesh:     mesh,
						modelMat: modelMat,
						distance: dist,
					})
					continue
				}
				opaqueItems = append(opaqueItems, renderItem{
					mesh:     mesh,
					modelMat: modelMat,
				})
			}
		}
	})

	if lights.droppedPoints > 0 {
		utils.Logger().Printf("Scene has %d point lights beyond the shader limit of %d; extras ignored",
			lights.droppedPoints, MaxPointLights)
	}
	a.computeLight(shader, &lights)

	if a.State.CollisionDebug && a.State.PlayerGravityMode {
		player := a.playerAABB(a.Camera.CameraPos)
		debugBoxes = append(debugBoxes, debugBox{
			min:   player.Min,
			max:   player.Max,
			color: mgl32.Vec3{0.2, 1.0, 0.2},
		})
	}

	for _, item := range opaqueItems {
		shader.SetMat4("model", item.modelMat)
		item.mesh.DrawPass(shader, false)
	}

	sort.Slice(transparentItems, func(i, j int) bool {
		return transparentItems[i].distance > transparentItems[j].distance
	})
	for _, item := range transparentItems {
		shader.SetMat4("model", item.modelMat)
		item.mesh.DrawPass(shader, true)
	}

	if a.State.CollisionDebug {
		a.debugBoxShader.Use()
		a.debugBoxShader.SetMat4("projection", a.Camera.ComputeProjection(a.width, a.height))
		a.debugBoxShader.SetMat4("view", a.Camera.ComputeView())
		gl.Disable(gl.CULL_FACE)
		for _, box := range debugBoxes {
			a.debugRenderer.Draw(a.debugBoxShader, box.min, box.max, box.color)
		}
		gl.Enable(gl.CULL_FACE)
	}

	a.skybox.RenderSkybox(a.Camera.ComputeView().Mat3().Mat4(), a.Camera.ComputeProjection(a.width, a.height))
}

func (a *App) fixedUpdate() {
	a.World.Write(func(entities []*Entity) {
		a.fixedUpdateComponents(entities)
		if a.State.GravityEnabled {
			a.stepBodies(entities)
		}
		a.stepPlayer(entities)
	})
}

// stepBodies integrates the dynamic entities and separates them from everything
// they end up overlapping.
func (a *App) stepBodies(entities []*Entity) {
	for _, entity := range entities {
		if entity.Body == nil || entity.Body.Static {
			continue
		}

		entity.Body.Velocity = entity.Body.Velocity.Add(a.gravityDirection.Mul(a.gravityStrength * a.physicsDeltaTime))
		entity.Translate(entity.Body.Velocity.Mul(a.physicsDeltaTime))

		for _, other := range entities {
			if other == entity {
				continue
			}

			box := entity.WorldAABB()
			otherBox := other.WorldAABB()
			if !box.Intersects(otherBox) {
				continue
			}

			separation := box.Separation(otherBox)
			if separation == (mgl32.Vec3{}) {
				continue
			}

			entity.Translate(separation)
			zeroVelocityOnSeparation(&entity.Body.Velocity, separation)
		}
	}
}

// stepPlayer moves the player capsule and resolves it against per-mesh bounds,
// which is finer-grained than the per-entity boxes used above.
func (a *App) stepPlayer(entities []*Entity) {
	if !a.State.PlayerGravityMode {
		a.playerVelocity = mgl32.Vec3{0, 0, 0}
		a.playerGrounded = false
		return
	}

	if a.State.GravityEnabled {
		a.playerVelocity = a.playerVelocity.Add(a.gravityDirection.Mul(a.gravityStrength * a.physicsDeltaTime))
	}

	// Reads the action rather than the key, so rebinding jump in config works
	// here too. IsDown rather than JustPressed: holding jump should keep
	// hopping, and the grounded check already gates repeats.
	if a.Input.IsDown(ActionJump) && a.playerGrounded && glfw.GetTime()-a.lastJumpTime >= 0.2 {
		a.playerVelocity = a.playerVelocity.Add(a.gravityDirection.Mul(-a.playerJumpSpeed))
		a.playerGrounded = false
		a.lastJumpTime = glfw.GetTime()
	}

	a.Camera.CameraPos = a.Camera.CameraPos.Add(a.playerVelocity.Mul(a.physicsDeltaTime))
	a.playerGrounded = false

	for _, entity := range entities {
		if entity.Renderer == nil || entity.Renderer.Model == nil {
			continue
		}

		modelMat := entity.WorldMatrix()
		for i := range entity.Renderer.Model.Meshes {
			meshBox := entity.Renderer.Model.Meshes[i].WorldAABB(modelMat)
			separation := a.playerAABB(a.Camera.CameraPos).Separation(meshBox)
			if separation == (mgl32.Vec3{}) {
				continue
			}

			a.Camera.CameraPos = a.Camera.CameraPos.Add(separation)
			zeroVelocityOnSeparation(&a.playerVelocity, separation)
			if separation.Dot(a.gravityDirection) < 0 {
				a.playerGrounded = true
			}
		}
	}
}

func zeroVelocityOnSeparation(velocity *mgl32.Vec3, separation mgl32.Vec3) {
	if separation.X() != 0 {
		(*velocity)[0] = 0
	}
	if separation.Y() != 0 {
		(*velocity)[1] = 0
	}
	if separation.Z() != 0 {
		(*velocity)[2] = 0
	}
}

// playerAABB is the capsule stand-in, an axis-aligned box hanging below the
// camera by playerCenterOffset.
func (a *App) playerAABB(cameraPos mgl32.Vec3) object.AABB {
	center := cameraPos.Add(a.playerCenterOffset)
	return object.AABB{
		Min: center.Sub(a.playerHalfExtents),
		Max: center.Add(a.playerHalfExtents),
	}
}

// appendDebugBox records a collision box if it is close enough to be worth
// drawing.
func (a *App) appendDebugBox(boxes *[]debugBox, box object.AABB, color mgl32.Vec3) {
	if a.Camera.CameraPos.Sub(box.Center()).Len() > a.collisionDebugDistance {
		return
	}
	*boxes = append(*boxes, debugBox{min: box.Min, max: box.Max, color: color})
}

// computeLight uploads the frame's lights. Everything here used to be
// hardcoded: one fixed directional light, nb_point_light pinned to 0 so the
// shader's point-light loop never ran, and a flashlight built from constants.
// It is all scene data now.
func (a *App) computeLight(shader *shaders.Shader, lights *lightSet) {
	if lights.directional != nil {
		shader.SetVec3Val("dirLight.direction", lights.directional.Direction)
		shader.SetVec3Val("dirLight.ambient", lights.directional.Ambient)
		shader.SetVec3Val("dirLight.diffuse", lights.directional.Diffuse)
		shader.SetVec3Val("dirLight.specular", lights.directional.Specular)
	} else {
		// No sun in the scene: contribute nothing rather than leaving whatever
		// the previous frame uploaded.
		shader.SetVec3("dirLight.direction", 0, -1, 0)
		shader.SetVec3("dirLight.ambient", 0, 0, 0)
		shader.SetVec3("dirLight.diffuse", 0, 0, 0)
		shader.SetVec3("dirLight.specular", 0, 0, 0)
	}

	shader.SetInt("nb_point_light", int32(len(lights.points)))
	for i, placed := range lights.points {
		prefix := "pointLights[" + strconv.Itoa(i) + "]."
		shader.SetVec3Val(prefix+"position", placed.position)
		shader.SetVec3Val(prefix+"ambient", placed.light.Ambient)
		shader.SetVec3Val(prefix+"diffuse", placed.light.Diffuse)
		shader.SetVec3Val(prefix+"specular", placed.light.Specular)
		shader.SetFloat(prefix+"constant", placed.light.Constant)
		shader.SetFloat(prefix+"linear", placed.light.Linear)
		shader.SetFloat(prefix+"quadratic", placed.light.Quadratic)
	}

	a.uploadSpotLight(shader, lights.spot)
}

func (a *App) uploadSpotLight(shader *shaders.Shader, placed *placedSpotLight) {
	if placed == nil {
		shader.SetBool("spotLight.isEnabled", false)
		return
	}

	light := placed.light
	position := placed.position
	direction := light.Direction

	if light.FollowCamera {
		// This is the flashlight: it rides the camera and the F key gates it.
		if !a.State.FlashLight {
			shader.SetBool("spotLight.isEnabled", false)
			return
		}
		position = a.Camera.CameraPos
		direction = a.Camera.CameraFront
	}

	if !light.Enabled {
		shader.SetBool("spotLight.isEnabled", false)
		return
	}

	shader.SetVec3Val("spotLight.position", position)
	shader.SetVec3Val("spotLight.direction", direction)
	shader.SetVec3Val("spotLight.ambient", light.Ambient)
	shader.SetVec3Val("spotLight.diffuse", light.Diffuse)
	shader.SetVec3Val("spotLight.specular", light.Specular)
	shader.SetFloat("spotLight.constant", light.Constant)
	shader.SetFloat("spotLight.linear", light.Linear)
	shader.SetFloat("spotLight.quadratic", light.Quadratic)
	shader.SetFloat("spotLight.cutOff", float32(math.Cos(float64(mgl32.DegToRad(light.CutOff)))))
	shader.SetFloat("spotLight.outerCutOff", float32(math.Cos(float64(mgl32.DegToRad(light.OuterCutOff)))))
	shader.SetBool("spotLight.isEnabled", true)
}

// processInput samples the keyboard once and lets handleActions react to the
// snapshot.
func (a *App) processInput() {
	a.Input.Poll(input.Window{Window: a.Window}, a.overlayCapturesKeyboard())
	a.handleActions()
}

// cycleGravityAxis advances to the next configured gravity direction.
func (a *App) cycleGravityAxis() {
	if len(a.gravityAxes) < 2 {
		return
	}

	for i, axis := range a.gravityAxes {
		if axis == a.gravityDirection {
			a.gravityDirection = a.gravityAxes[(i+1)%len(a.gravityAxes)]
			utils.Logger().Println("Gravity axis set to", a.gravityDirection)
			return
		}
	}

	a.gravityDirection = a.gravityAxes[0]
	utils.Logger().Println("Gravity axis set to", a.gravityDirection)
}

func (a *App) onFramebufferSize(window *glfw.Window, width, height int) {
	a.width = width
	a.height = height

	gl.Viewport(0, 0, int32(width), int32(height))
}

func (a *App) fpsCounter() {
	delta := a.lastFrame - a.lastFrameCounter
	a.nbFrames++

	if delta >= 1.0 {
		fps := float64(a.nbFrames) / float64(delta)
		a.Window.SetTitle(a.opts.Title + " - FPS: " + strconv.FormatFloat(fps, 'f', 2, 64))
		a.nbFrames = 0
		a.lastFrameCounter = float32(glfw.GetTime())
	}
}

// resetDynamicState clears the player physics and returns the camera to the
// current scene's spawn point.
func (a *App) resetDynamicState() {
	a.playerVelocity = mgl32.Vec3{0, 0, 0}
	a.playerGrounded = false
	a.lastJumpTime = 0

	if a.Camera == nil {
		return
	}

	spawn := a.Scenes.CameraSpawn()
	a.Camera.CameraPos = mgl32.Vec3(spawn.Position)
	a.Camera.SetOrientation(spawn.Yaw, spawn.Pitch)
}

// setSkybox swaps the cubemap when a scene asks for a different one. The old
// cubemap is deleted, since nothing else references it.
func (a *App) setSkybox(path string) error {
	if path == "" || a.skybox == nil || a.skybox.Path == path {
		return nil
	}

	shader := a.skybox.Shader
	a.skybox.Delete()

	a.skybox = object.CreateSkybox(path)
	a.skybox.Shader = shader
	a.skybox.LoadCubemap()
	a.skybox.Shader.SetInt("skybox", int32(a.skybox.SkyboxTextureUnit))
	return nil
}
