package main

import (
	"3d-engine/camera"
	"3d-engine/object"
	"3d-engine/scene"
	"3d-engine/shaders"
	"3d-engine/textures"
	"3d-engine/utils"
	"math"
	"runtime"
	"sort"
	"strconv"
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

	models []*object.Model

	kh = camera.NewKeyHandler()
)

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

	scene, err := scene.Load(utils.GetContext().ScenePath)
	if err != nil {
		utils.Logger().Fatalln("Could not load scene:", err)
	}

	go StartRPCServer()

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

	lightingShader, err := shaders.CreateShaderProgram("lighting.vert", "lighting.frag")
	if err != nil {
		utils.Logger().Fatalln("Could not create cube shader:", err)
	}
	defer lightingShader.Delete()

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

	modelId := uint32(0)
	for _, obj := range scene.Objects {
		model := object.Model{Id: modelId, Name: obj.Name}
		model.LoadScene(obj.Path)

		model.Coordinates = mgl32.Vec3{obj.OriginX, obj.OriginY, obj.OriginZ}
		model.Rotation = mgl32.Vec4{obj.RotationX, obj.RotationY, obj.RotationZ, obj.RotationAngle}
		model.Scale = mgl32.Vec3{obj.ScaleX, obj.ScaleY, obj.ScaleZ}

		models = append(models, &model)
		modelId++
	}

	cam := camera.NewCamera(config)

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

		select {
		case <-ticker.C:
			fixedUpdate(window)
		default:
		}

		update(lightingShader, cam, models, skybox)

		window.SwapBuffers()
		glfw.PollEvents()
	}
}

func update(shader *shaders.Shader, cam *camera.Camera, models []*object.Model, skybox *object.Skybox) {
	// Lighting
	shader.Use()
	shader.SetVec3Val("viewPos", cam.CameraPos)

	projection := cam.ComputeProjection(g_width, g_height)
	shader.SetMat4("projection", projection)
	view := cam.ComputeView()
	shader.SetMat4("view", view)

	computeLight(shader, cam)

	sort.Slice(models, func(i, j int) bool {
		return cam.CameraPos.Sub(models[i].Coordinates).LenSqr() > cam.CameraPos.Sub(models[j].Coordinates).LenSqr()
	})

	shader.SetInt("skybox", int32(skybox.SkyboxTextureUnit))

	for _, model := range models {
		modelVec := mgl32.Ident4()
		modelVec = modelVec.Mul4(mgl32.Translate3D(model.Coordinates.X(), model.Coordinates.Y(), model.Coordinates.Z()))
		modelVec = modelVec.Mul4(mgl32.HomogRotate3D(mgl32.DegToRad(model.Rotation.W()), model.Rotation.Vec3()))
		modelVec = modelVec.Mul4(mgl32.Scale3D(model.Scale.X(), model.Scale.Y(), model.Scale.Z()))
		shader.SetMat4("model", modelVec)
		model.Draw(shader)
	}

	skybox.RenderSkybox(cam.ComputeView().Mat3().Mat4(), cam.ComputeProjection(g_width, g_height))
}

func fixedUpdate(window *glfw.Window) {

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
