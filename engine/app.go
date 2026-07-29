// Package engine hosts the runtime: window/GL setup, the frame loop, the model
// list and the systems that act on it. A game is expected to build an App with
// New, optionally drive it through the exported API, and call Run.
package engine

import (
	"3d-engine/camera"
	"3d-engine/object"
	"3d-engine/shaders"
	"3d-engine/textures"
	"3d-engine/utils"
	"fmt"
	"math"
	"sort"
	"strconv"
	"sync"
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
	if o.RPCAddr == "" {
		o.RPCAddr = "localhost:8080"
	}
}

// App owns every piece of runtime state that used to live as a package-level
// var in main. All GL work happens on the goroutine that calls Run.
type App struct {
	opts   Options
	Config *utils.Config

	Window *glfw.Window
	Camera *camera.Camera
	Scenes *SceneManager
	Keys   *camera.KeyHandler

	width  int
	height int

	deltaTime        float32
	lastFrame        float32
	lastFrameCounter float32
	nbFrames         int

	models   []*object.Model
	modelsMu sync.RWMutex

	lightingShader *shaders.Shader
	debugBoxShader *shaders.Shader
	debugRenderer  *debugBoxRenderer
	skybox         *object.Skybox

	physicsDeltaTime float32
	gravityStrength  float32
	gravityDirection mgl32.Vec3

	lastGravityAxisToggle float64

	playerVelocity     mgl32.Vec3
	playerHalfExtents  mgl32.Vec3
	playerCenterOffset mgl32.Vec3
	playerJumpSpeed    float32
	playerGrounded     bool
	lastJumpTime       float64

	collisionDebugDistance float32

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
		Keys:   camera.NewKeyHandler(),

		physicsDeltaTime: 1.0 / float32(fixedUpdateRate),
		gravityStrength:  9.81,
		gravityDirection: mgl32.Vec3{0.0, -1.0, 0.0},

		playerHalfExtents:  mgl32.Vec3{0.35, 0.9, 0.35},
		playerCenterOffset: mgl32.Vec3{0.0, -0.9, 0.0},
		playerJumpSpeed:    6.0,

		collisionDebugDistance: 80.0,
	}
	a.Scenes = NewSceneManager(a, config, opts.ScenePath)

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

	a.Window.SetCursorPosCallback(a.Camera.MouseCallback)
	a.Window.SetScrollCallback(a.Camera.ScrollCallback)
	a.Keys.RegisterKeys(a.Window, a.Camera, &a.deltaTime)

	if !opts.DisableRPC {
		if err := a.startRPCServer(opts.RPCAddr); err != nil {
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
	window.SetInputMode(glfw.CursorMode, glfw.CursorDisabled)

	ctx := utils.GetContext()
	ctx.CaptureCursor = true
	ctx.GravityEnabled = true
	ctx.PlayerGravityMode = false
	ctx.CollisionDebug = false

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

// Run drives the frame loop until the window is closed.
func (a *App) Run() error {
	fixedDeltaTime := time.Second / time.Duration(fixedUpdateRate)
	ticker := time.NewTicker(fixedDeltaTime)
	defer ticker.Stop()

	for !a.Window.ShouldClose() {
		currentFrame := float32(glfw.GetTime())
		a.deltaTime = currentFrame - a.lastFrame
		a.lastFrame = currentFrame

		if utils.GetContext().DebugLevel > utils.Info {
			utils.Logger().Printf("Frame time: %.2f ms\n", a.deltaTime*1000)
		}

		a.fpsCounter()

		a.processInput()

		gl.ClearColor(0.0, 0.0, 0.0, 1.0)
		gl.Clear(gl.COLOR_BUFFER_BIT | gl.DEPTH_BUFFER_BIT)

		changed, err := a.Scenes.ApplyPendingSceneChange()
		if err != nil {
			utils.Logger().Printf("Failed to switch scene: %v", err)
		} else if changed {
			utils.Logger().Printf("Switched scene to %s", a.Scenes.CurrentScenePath())
			a.resetDynamicState()
		}

		select {
		case <-ticker.C:
			a.fixedUpdate()
		default:
		}

		a.render()

		a.Window.SwapBuffers()
		glfw.PollEvents()
	}

	return nil
}

// Close releases the GL resources, the window and the RPC listener. It is safe
// to call on a partially constructed App.
func (a *App) Close() {
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
func (a *App) Models(fn func(models []*object.Model)) {
	a.modelsMu.RLock()
	defer a.modelsMu.RUnlock()
	fn(a.models)
}

// MutateModel runs fn against the model with the given id under the write lock
// and reports whether it was found.
func (a *App) MutateModel(id uint32, fn func(model *object.Model)) bool {
	a.modelsMu.Lock()
	defer a.modelsMu.Unlock()

	for _, model := range a.models {
		if model.Id == id {
			fn(model)
			return true
		}
	}
	return false
}

func (a *App) setModels(models []*object.Model) {
	a.modelsMu.Lock()
	a.models = models
	a.modelsMu.Unlock()
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

	a.computeLight(shader)

	shader.SetInt("skybox", int32(a.skybox.SkyboxTextureUnit))

	opaqueItems := make([]renderItem, 0)
	transparentItems := make([]renderItem, 0)
	debugBoxes := make([]debugBox, 0)

	a.modelsMu.RLock()
	for _, model := range a.models {
		modelVec := mgl32.Ident4()
		modelVec = modelVec.Mul4(mgl32.Translate3D(model.Coordinates.X(), model.Coordinates.Y(), model.Coordinates.Z()))
		modelVec = modelVec.Mul4(mgl32.HomogRotate3D(mgl32.DegToRad(model.Rotation.W()), model.Rotation.Vec3()))
		modelVec = modelVec.Mul4(mgl32.Scale3D(model.Scale.X(), model.Scale.Y(), model.Scale.Z()))

		if utils.GetContext().CollisionDebug {
			modelMin, modelMax := model.WorldAABB()
			modelCenter := modelMin.Add(modelMax).Mul(0.5)
			if a.Camera.CameraPos.Sub(modelCenter).Len() <= a.collisionDebugDistance {
				debugBoxes = append(debugBoxes, debugBox{
					min:   modelMin,
					max:   modelMax,
					color: mgl32.Vec3{1.0, 0.2, 0.2},
				})
			}
		}

		for i := range model.Meshes {
			mesh := &model.Meshes[i]
			if utils.GetContext().CollisionDebug {
				meshMin, meshMax := mesh.WorldAABB(modelVec)
				meshCenter := meshMin.Add(meshMax).Mul(0.5)
				if a.Camera.CameraPos.Sub(meshCenter).Len() <= a.collisionDebugDistance {
					debugBoxes = append(debugBoxes, debugBox{
						min:   meshMin,
						max:   meshMax,
						color: mgl32.Vec3{1.0, 0.8, 0.2},
					})
				}
			}

			if mesh.IsTransparent() {
				dist := a.Camera.CameraPos.Sub(mesh.WorldCenter(modelVec)).LenSqr()
				transparentItems = append(transparentItems, renderItem{
					mesh:     mesh,
					modelMat: modelVec,
					distance: dist,
				})
				continue
			}
			opaqueItems = append(opaqueItems, renderItem{
				mesh:     mesh,
				modelMat: modelVec,
			})
		}
	}
	a.modelsMu.RUnlock()

	if utils.GetContext().CollisionDebug && utils.GetContext().PlayerGravityMode {
		playerMin, playerMax := a.playerAABB(a.Camera.CameraPos)
		debugBoxes = append(debugBoxes, debugBox{
			min:   playerMin,
			max:   playerMax,
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

	if utils.GetContext().CollisionDebug {
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
	a.modelsMu.Lock()
	defer a.modelsMu.Unlock()

	if utils.GetContext().GravityEnabled {
		for _, model := range a.models {
			if model.IsStatic {
				continue
			}

			model.Velocity = model.Velocity.Add(a.gravityDirection.Mul(a.gravityStrength * a.physicsDeltaTime))
			model.Coordinates = model.Coordinates.Add(model.Velocity.Mul(a.physicsDeltaTime))

			for _, other := range a.models {
				if other.Id == model.Id {
					continue
				}
				if !model.Intersects(other) {
					continue
				}

				separation := model.CollisionSeparation(other)
				if separation == (mgl32.Vec3{}) {
					continue
				}

				model.Coordinates = model.Coordinates.Add(separation)
				zeroVelocityOnSeparation(&model.Velocity, separation)
			}
		}
	}

	if !utils.GetContext().PlayerGravityMode {
		a.playerVelocity = mgl32.Vec3{0, 0, 0}
		a.playerGrounded = false
		return
	}

	if utils.GetContext().GravityEnabled {
		a.playerVelocity = a.playerVelocity.Add(a.gravityDirection.Mul(a.gravityStrength * a.physicsDeltaTime))
	}

	if a.Window.GetKey(glfw.KeySpace) == glfw.Press && a.playerGrounded && glfw.GetTime()-a.lastJumpTime >= 0.2 {
		a.playerVelocity = a.playerVelocity.Add(a.gravityDirection.Mul(-a.playerJumpSpeed))
		a.playerGrounded = false
		a.lastJumpTime = glfw.GetTime()
	}

	a.Camera.CameraPos = a.Camera.CameraPos.Add(a.playerVelocity.Mul(a.physicsDeltaTime))
	a.playerGrounded = false

	for _, model := range a.models {
		modelMat := mgl32.Ident4()
		modelMat = modelMat.Mul4(mgl32.Translate3D(model.Coordinates.X(), model.Coordinates.Y(), model.Coordinates.Z()))
		modelMat = modelMat.Mul4(mgl32.HomogRotate3D(mgl32.DegToRad(model.Rotation.W()), model.Rotation.Vec3()))
		modelMat = modelMat.Mul4(mgl32.Scale3D(model.Scale.X(), model.Scale.Y(), model.Scale.Z()))

		for i := range model.Meshes {
			meshMin, meshMax := model.Meshes[i].WorldAABB(modelMat)
			separation := a.playerAABBCollisionSeparation(a.Camera.CameraPos, meshMin, meshMax)
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

func (a *App) playerAABB(cameraPos mgl32.Vec3) (mgl32.Vec3, mgl32.Vec3) {
	center := cameraPos.Add(a.playerCenterOffset)
	return center.Sub(a.playerHalfExtents), center.Add(a.playerHalfExtents)
}

func (a *App) playerAABBCollisionSeparation(cameraPos mgl32.Vec3, otherMin, otherMax mgl32.Vec3) mgl32.Vec3 {
	playerMin, playerMax := a.playerAABB(cameraPos)

	overlapX := minf(playerMax.X(), otherMax.X()) - maxf(playerMin.X(), otherMin.X())
	overlapY := minf(playerMax.Y(), otherMax.Y()) - maxf(playerMin.Y(), otherMin.Y())
	overlapZ := minf(playerMax.Z(), otherMax.Z()) - maxf(playerMin.Z(), otherMin.Z())
	if overlapX <= 0 || overlapY <= 0 || overlapZ <= 0 {
		return mgl32.Vec3{0, 0, 0}
	}

	playerCenter := playerMin.Add(playerMax).Mul(0.5)
	otherCenter := otherMin.Add(otherMax).Mul(0.5)

	if overlapX <= overlapY && overlapX <= overlapZ {
		if playerCenter.X() < otherCenter.X() {
			return mgl32.Vec3{-overlapX, 0, 0}
		}
		return mgl32.Vec3{overlapX, 0, 0}
	}

	if overlapY <= overlapX && overlapY <= overlapZ {
		if playerCenter.Y() < otherCenter.Y() {
			return mgl32.Vec3{0, -overlapY, 0}
		}
		return mgl32.Vec3{0, overlapY, 0}
	}

	if playerCenter.Z() < otherCenter.Z() {
		return mgl32.Vec3{0, 0, -overlapZ}
	}
	return mgl32.Vec3{0, 0, overlapZ}
}

func minf(a, b float32) float32 {
	if a < b {
		return a
	}
	return b
}

func maxf(a, b float32) float32 {
	if a > b {
		return a
	}
	return b
}

func (a *App) computeLight(shader *shaders.Shader) {
	// Directional light
	shader.SetVec3("dirLight.direction", -0.2, -1.0, -0.3)
	shader.SetVec3("dirLight.ambient", 0.2, 0.2, 0.2)
	shader.SetVec3("dirLight.diffuse", 0.5, 0.5, 0.5)
	shader.SetVec3("dirLight.specular", 1.0, 1.0, 1.0)

	shader.SetInt("nb_point_light", 0)
	// Point light
	// for i, pointLightPos := range pointLightPositions {
	// 	lightingShader.SetVec3Val(fmt.Sprintf("pointLights[%d].position", i), pointLightPos)
	// 	lightingShader.SetVec3(fmt.Sprintf("pointLights[%d].ambiant", i), 0.05, 0.05, 0.05)
	// 	lightingShader.SetVec3(fmt.Sprintf("pointLights[%d].diffuse", i), 0.8, 0.8, 0.8)
	// 	lightingShader.SetVec3(fmt.Sprintf("pointLights[%d].specular", i), 1.0, 1.0, 1.0)
	// 	lightingShader.SetFloat(fmt.Sprintf("pointLights[%d].constant", i), 1.0)
	// 	lightingShader.SetFloat(fmt.Sprintf("pointLights[%d].linear", i), 0.09)
	// 	lightingShader.SetFloat(fmt.Sprintf("pointLights[%d].quadratic", i), 0.032)
	// }

	// Spot light
	if utils.GetContext().FlashLight {
		shader.SetVec3Val("spotLight.position", a.Camera.CameraPos)
		shader.SetVec3Val("spotLight.direction", a.Camera.CameraFront)
		shader.SetVec3("spotLight.ambient", 0.0, 0.0, 0.0)
		shader.SetVec3("spotLight.diffuse", 1.0, 1.0, 1.0)
		shader.SetVec3("spotLight.specular", 1.0, 1.0, 1.0)
		shader.SetFloat("spotLight.constant", 1.0)
		shader.SetFloat("spotLight.linear", 0.09)
		shader.SetFloat("spotLight.quadratic", 0.032)
		shader.SetFloat("spotLight.cutOff", float32(math.Cos(float64(mgl32.DegToRad(12.5)))))
		shader.SetFloat("spotLight.outerCutOff", float32(math.Cos(float64(mgl32.DegToRad(15.0)))))
		shader.SetBool("spotLight.isEnabled", true)
	} else {
		shader.SetBool("spotLight.isEnabled", false)
	}
}

func (a *App) processInput() {
	// Switch gravity axis between -Y and -Z for world-space testing.
	if a.Window.GetKey(glfw.KeyH) == glfw.Press && glfw.GetTime()-a.lastGravityAxisToggle >= 0.3 {
		a.lastGravityAxisToggle = glfw.GetTime()
		if a.gravityDirection == (mgl32.Vec3{0.0, -1.0, 0.0}) {
			a.gravityDirection = mgl32.Vec3{0.0, 0.0, -1.0}
			utils.Logger().Println("Gravity axis set to -Z")
		} else {
			a.gravityDirection = mgl32.Vec3{0.0, -1.0, 0.0}
			utils.Logger().Println("Gravity axis set to -Y")
		}
	}

	for i := glfw.KeySpace; i < glfw.KeyLast; i++ {
		if a.Window.GetKey(i) == glfw.Press {
			a.Keys.PressKey(i)
			a.Keys.IsPressed[i] = true
		} else if a.Window.GetKey(i) == glfw.Release {
			a.Keys.IsPressed[i] = false
		}
	}
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

func (a *App) resetDynamicState() {
	a.playerVelocity = mgl32.Vec3{0, 0, 0}
	a.playerGrounded = false
	a.lastJumpTime = 0
	if a.Camera != nil {
		a.Camera.CameraPos = mgl32.Vec3{0.0, 0.0, 3.0}
	}
}
