package main

import (
	"3d-engine/camera"
	"3d-engine/object"
	"3d-engine/shaders"
	"3d-engine/textures"
	"3d-engine/utils"
	"math"
	"runtime"
	"sort"
	"strconv"
	"sync"
	"time"

	"github.com/go-gl/gl/v4.6-core/gl"
	"github.com/go-gl/glfw/v3.3/glfw"
	"github.com/go-gl/mathgl/mgl32"
)

var (
	g_width  = 800
	g_height = 600

	// Delta clasic
	deltaTime float32 = 0.0
	lastFrame float32 = 0.0

	// FPS counter
	lastFrameCounter float32 = 0.0
	nbFrames                 = 0

	config *utils.Config

	models   []*object.Model
	modelsMu sync.RWMutex
	sceneMgr *SceneManager

	kh = camera.NewKeyHandler()

	physicsDeltaTime float32 = 1.0 / 50.0
	gravityStrength  float32 = 9.81
	gravityDirection         = mgl32.Vec3{0.0, -1.0, 0.0}

	lastGravityAxisToggle float64 = 0

	playerVelocity             = mgl32.Vec3{0, 0, 0}
	playerHalfExtents          = mgl32.Vec3{0.35, 0.9, 0.35}
	playerCenterOffset         = mgl32.Vec3{0.0, -0.9, 0.0}
	playerJumpSpeed    float32 = 6.0
	playerGrounded             = false
	lastJumpTime       float64 = 0

	collisionDebugDistance float32 = 80.0
)

type debugBox struct {
	min   mgl32.Vec3
	max   mgl32.Vec3
	color mgl32.Vec3
}

type debugBoxRenderer struct {
	vao uint32
	vbo uint32
}

func newDebugBoxRenderer() *debugBoxRenderer {
	r := &debugBoxRenderer{}

	vertices := []float32{
		-0.5, -0.5, -0.5, 0.5, -0.5, -0.5,
		0.5, -0.5, -0.5, 0.5, 0.5, -0.5,
		0.5, 0.5, -0.5, -0.5, 0.5, -0.5,
		-0.5, 0.5, -0.5, -0.5, -0.5, -0.5,

		-0.5, -0.5, 0.5, 0.5, -0.5, 0.5,
		0.5, -0.5, 0.5, 0.5, 0.5, 0.5,
		0.5, 0.5, 0.5, -0.5, 0.5, 0.5,
		-0.5, 0.5, 0.5, -0.5, -0.5, 0.5,

		-0.5, -0.5, -0.5, -0.5, -0.5, 0.5,
		0.5, -0.5, -0.5, 0.5, -0.5, 0.5,
		0.5, 0.5, -0.5, 0.5, 0.5, 0.5,
		-0.5, 0.5, -0.5, -0.5, 0.5, 0.5,
	}

	gl.GenVertexArrays(1, &r.vao)
	gl.GenBuffers(1, &r.vbo)
	gl.BindVertexArray(r.vao)
	gl.BindBuffer(gl.ARRAY_BUFFER, r.vbo)
	gl.BufferData(gl.ARRAY_BUFFER, len(vertices)*4, gl.Ptr(vertices), gl.STATIC_DRAW)
	gl.EnableVertexAttribArray(0)
	gl.VertexAttribPointer(0, 3, gl.FLOAT, false, 3*4, nil)
	gl.BindVertexArray(0)

	return r
}

func (r *debugBoxRenderer) Delete() {
	gl.DeleteVertexArrays(1, &r.vao)
	gl.DeleteBuffers(1, &r.vbo)
}

func (r *debugBoxRenderer) Draw(shader *shaders.Shader, min, max, color mgl32.Vec3) {
	center := min.Add(max).Mul(0.5)
	size := max.Sub(min)

	modelMat := mgl32.Ident4()
	modelMat = modelMat.Mul4(mgl32.Translate3D(center.X(), center.Y(), center.Z()))
	modelMat = modelMat.Mul4(mgl32.Scale3D(size.X(), size.Y(), size.Z()))

	shader.SetMat4("model", modelMat)
	shader.SetVec3Val("color", color)
	gl.BindVertexArray(r.vao)
	gl.DrawArrays(gl.LINES, 0, 24)
	gl.BindVertexArray(0)
}

func init() {
	runtime.LockOSThread()
}

func main() {
	var err error

	utils.ParseArgs()
	config, err = utils.LoadConfig(utils.GetContext().ConfigPath)
	if err != nil {
		utils.Logger().Fatalln("Could not load config:", err)
	}
	sceneMgr = NewSceneManager(config, utils.GetContext().ScenePath)

	g_width = config.Width
	g_height = config.Height

	if err := glfw.Init(); err != nil {
		utils.Logger().Fatalln("Could not init glfw:", err)
	}
	defer glfw.Terminate()

	glfw.WindowHint(glfw.ContextVersionMajor, 4)
	glfw.WindowHint(glfw.ContextVersionMinor, 6)
	glfw.WindowHint(glfw.OpenGLProfile, glfw.OpenGLCoreProfile)

	window, err := glfw.CreateWindow(g_width, g_height, "3D-Engine", nil, nil)
	if err != nil {
		utils.Logger().Fatalln("Could not create a window:", err)
	}
	defer window.Destroy()

	window.MakeContextCurrent()

	if err := gl.Init(); err != nil {
		utils.Logger().Fatalln("Failed to initialize OpenGL:", err)
	}

	gl.Viewport(0, 0, int32(g_width), int32(g_height))

	gl.Enable(gl.DEPTH_TEST)
	gl.Enable(gl.CULL_FACE)

	gl.CullFace(gl.BACK)
	gl.FrontFace(gl.CCW)

	glfw.SwapInterval(config.GetVsync())

	window.SetFramebufferSizeCallback(framebufferSizeCallback)

	window.SetInputMode(glfw.CursorMode, glfw.CursorDisabled)
	utils.GetContext().CaptureCursor = true
	utils.GetContext().GravityEnabled = true
	utils.GetContext().PlayerGravityMode = false
	utils.GetContext().CollisionDebug = false

	lightingShader, err := shaders.CreateShaderProgram("lighting.vert", "lighting.frag")
	if err != nil {
		utils.Logger().Fatalln("Could not create cube shader:", err)
	}
	defer lightingShader.Delete()

	debugBoxShader, err := shaders.CreateShaderProgram("debug_box.vert", "debug_box.frag")
	if err != nil {
		utils.Logger().Fatalln("Could not create debug shader:", err)
	}
	defer debugBoxShader.Delete()
	debugRenderer := newDebugBoxRenderer()
	defer debugRenderer.Delete()

	transparent := false
	lightingShader.NoTexture, err = textures.Load("./textures/missing.png", &transparent)
	if err != nil {
		utils.Logger().Fatalln("Could not load missing texture", err)
	}

	skybox := object.CreateSkybox("./testObjects/skybox")
	skybox.LoadCubemap()
	skybox.Shader, err = shaders.CreateShaderProgram("skybox.vert", "skybox.frag")
	if err != nil {
		utils.Logger().Fatalln("Could not create skybox shader:", err)
	}
	defer skybox.Shader.Delete()
	skybox.Shader.SetInt("skybox", int32(skybox.SkyboxTextureUnit))

	// lightSourceShader, err := shaders.CreateShaderProgram("lighting.vert", "lightSource.frag")
	// if err != nil {
	// 	log.Fatalln("Could not create light shader:", err)
	// }
	// defer lightSourceShader.Delete()

	cam := camera.NewCamera(config)
	if err := sceneMgr.LoadScene(sceneMgr.ResolveInitialScenePath()); err != nil {
		utils.Logger().Fatalln("Could not load scene:", err)
	}
	resetDynamicState(cam)

	go StartRPCServer()

	window.SetCursorPosCallback(cam.MouseCallback)
	window.SetScrollCallback(cam.ScrollCallback)

	kh.RegisterKeys(window, cam, &deltaTime)

	const fixedUpdateRate = 50
	fixedDeltaTime := time.Second / time.Duration(fixedUpdateRate)
	ticker := time.NewTicker(fixedDeltaTime)
	defer ticker.Stop()

	for !window.ShouldClose() {
		currentFrame := float32(glfw.GetTime())
		deltaTime = currentFrame - lastFrame
		lastFrame = currentFrame

		if utils.GetContext().DebugLevel > utils.Info {
			utils.Logger().Printf("Frame time: %.2f ms\n", deltaTime*1000)
		}

		fpsCounter(window)

		processInput(window)

		gl.ClearColor(0.0, 0.0, 0.0, 1.0)
		gl.Clear(gl.COLOR_BUFFER_BIT | gl.DEPTH_BUFFER_BIT)

		changed, err := sceneMgr.ApplyPendingSceneChange()
		if err != nil {
			utils.Logger().Printf("Failed to switch scene: %v", err)
		} else if changed {
			utils.Logger().Printf("Switched scene to %s", sceneMgr.CurrentScenePath())
			resetDynamicState(cam)
		}

		select {
		case <-ticker.C:
			fixedUpdate(window, cam)
		default:
		}

		update(lightingShader, debugBoxShader, debugRenderer, cam, models, skybox)

		window.SwapBuffers()
		glfw.PollEvents()
	}
}

func update(shader *shaders.Shader, debugShader *shaders.Shader, debugRenderer *debugBoxRenderer, cam *camera.Camera, models []*object.Model, skybox *object.Skybox) {
	type renderItem struct {
		mesh     *object.Mesh
		modelMat mgl32.Mat4
		distance float32
	}

	// Lighting
	shader.Use()
	shader.SetVec3Val("viewPos", cam.CameraPos)

	projection := cam.ComputeProjection(g_width, g_height)
	shader.SetMat4("projection", projection)
	view := cam.ComputeView()
	shader.SetMat4("view", view)

	computeLight(shader, cam)

	shader.SetInt("skybox", int32(skybox.SkyboxTextureUnit))

	opaqueItems := make([]renderItem, 0)
	transparentItems := make([]renderItem, 0)
	debugBoxes := make([]debugBox, 0)

	modelsMu.RLock()
	for _, model := range models {
		modelVec := mgl32.Ident4()
		modelVec = modelVec.Mul4(mgl32.Translate3D(model.Coordinates.X(), model.Coordinates.Y(), model.Coordinates.Z()))
		modelVec = modelVec.Mul4(mgl32.HomogRotate3D(mgl32.DegToRad(model.Rotation.W()), model.Rotation.Vec3()))
		modelVec = modelVec.Mul4(mgl32.Scale3D(model.Scale.X(), model.Scale.Y(), model.Scale.Z()))

		if utils.GetContext().CollisionDebug {
			modelMin, modelMax := model.WorldAABB()
			modelCenter := modelMin.Add(modelMax).Mul(0.5)
			if cam.CameraPos.Sub(modelCenter).Len() <= collisionDebugDistance {
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
				if cam.CameraPos.Sub(meshCenter).Len() <= collisionDebugDistance {
					debugBoxes = append(debugBoxes, debugBox{
						min:   meshMin,
						max:   meshMax,
						color: mgl32.Vec3{1.0, 0.8, 0.2},
					})
				}
			}

			if mesh.IsTransparent() {
				dist := cam.CameraPos.Sub(mesh.WorldCenter(modelVec)).LenSqr()
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
	modelsMu.RUnlock()

	if utils.GetContext().CollisionDebug && utils.GetContext().PlayerGravityMode {
		playerMin, playerMax := playerAABB(cam.CameraPos)
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
		debugShader.Use()
		debugShader.SetMat4("projection", cam.ComputeProjection(g_width, g_height))
		debugShader.SetMat4("view", cam.ComputeView())
		gl.Disable(gl.CULL_FACE)
		for _, box := range debugBoxes {
			debugRenderer.Draw(debugShader, box.min, box.max, box.color)
		}
		gl.Enable(gl.CULL_FACE)
	}

	skybox.RenderSkybox(cam.ComputeView().Mat3().Mat4(), cam.ComputeProjection(g_width, g_height))
}

func fixedUpdate(window *glfw.Window, cam *camera.Camera) {
	modelsMu.Lock()
	defer modelsMu.Unlock()

	if utils.GetContext().GravityEnabled {
		for _, model := range models {
			if model.IsStatic {
				continue
			}

			model.Velocity = model.Velocity.Add(gravityDirection.Mul(gravityStrength * physicsDeltaTime))
			model.Coordinates = model.Coordinates.Add(model.Velocity.Mul(physicsDeltaTime))

			for _, other := range models {
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
		playerVelocity = mgl32.Vec3{0, 0, 0}
		playerGrounded = false
		return
	}

	if utils.GetContext().GravityEnabled {
		playerVelocity = playerVelocity.Add(gravityDirection.Mul(gravityStrength * physicsDeltaTime))
	}

	if window.GetKey(glfw.KeySpace) == glfw.Press && playerGrounded && glfw.GetTime()-lastJumpTime >= 0.2 {
		playerVelocity = playerVelocity.Add(gravityDirection.Mul(-playerJumpSpeed))
		playerGrounded = false
		lastJumpTime = glfw.GetTime()
	}

	cam.CameraPos = cam.CameraPos.Add(playerVelocity.Mul(physicsDeltaTime))
	playerGrounded = false

	for _, model := range models {
		modelMat := mgl32.Ident4()
		modelMat = modelMat.Mul4(mgl32.Translate3D(model.Coordinates.X(), model.Coordinates.Y(), model.Coordinates.Z()))
		modelMat = modelMat.Mul4(mgl32.HomogRotate3D(mgl32.DegToRad(model.Rotation.W()), model.Rotation.Vec3()))
		modelMat = modelMat.Mul4(mgl32.Scale3D(model.Scale.X(), model.Scale.Y(), model.Scale.Z()))

		for i := range model.Meshes {
			meshMin, meshMax := model.Meshes[i].WorldAABB(modelMat)
			separation := playerAABBCollisionSeparation(cam.CameraPos, meshMin, meshMax)
			if separation == (mgl32.Vec3{}) {
				continue
			}

			cam.CameraPos = cam.CameraPos.Add(separation)
			zeroVelocityOnSeparation(&playerVelocity, separation)
			if separation.Dot(gravityDirection) < 0 {
				playerGrounded = true
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

func playerAABB(cameraPos mgl32.Vec3) (mgl32.Vec3, mgl32.Vec3) {
	center := cameraPos.Add(playerCenterOffset)
	return center.Sub(playerHalfExtents), center.Add(playerHalfExtents)
}

func playerAABBCollisionSeparation(cameraPos mgl32.Vec3, otherMin, otherMax mgl32.Vec3) mgl32.Vec3 {
	playerMin, playerMax := playerAABB(cameraPos)

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

func computeLight(shader *shaders.Shader, cam *camera.Camera) {
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
		shader.SetVec3Val("spotLight.position", cam.CameraPos)
		shader.SetVec3Val("spotLight.direction", cam.CameraFront)
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

func processInput(window *glfw.Window) {
	// Switch gravity axis between -Y and -Z for world-space testing.
	if window.GetKey(glfw.KeyH) == glfw.Press && glfw.GetTime()-lastGravityAxisToggle >= 0.3 {
		lastGravityAxisToggle = glfw.GetTime()
		if gravityDirection == (mgl32.Vec3{0.0, -1.0, 0.0}) {
			gravityDirection = mgl32.Vec3{0.0, 0.0, -1.0}
			utils.Logger().Println("Gravity axis set to -Z")
		} else {
			gravityDirection = mgl32.Vec3{0.0, -1.0, 0.0}
			utils.Logger().Println("Gravity axis set to -Y")
		}
	}

	for i := glfw.KeySpace; i < glfw.KeyLast; i++ {
		if window.GetKey(i) == glfw.Press {
			kh.PressKey(i)
			kh.IsPressed[i] = true
		} else if window.GetKey(i) == glfw.Release {
			kh.IsPressed[i] = false
		}
	}
}

func framebufferSizeCallback(window *glfw.Window, width, height int) {
	g_width = width
	g_height = height

	gl.Viewport(0, 0, int32(width), int32(height))
}

func fpsCounter(window *glfw.Window) {
	delta := lastFrame - lastFrameCounter
	nbFrames++

	if delta >= 1.0 {
		fps := float64(nbFrames) / float64(delta)
		window.SetTitle("3D-Engine - FPS: " + strconv.FormatFloat(fps, 'f', 2, 64))
		nbFrames = 0
		lastFrameCounter = float32(glfw.GetTime())
	}
}

func resetDynamicState(cam *camera.Camera) {
	playerVelocity = mgl32.Vec3{0, 0, 0}
	playerGrounded = false
	lastJumpTime = 0
	if cam != nil {
		cam.CameraPos = mgl32.Vec3{0.0, 0.0, 3.0}
	}
}
