package main

import (
	"3d-engine/camera"
	"3d-engine/loader"
	"3d-engine/shaders"
	"3d-engine/textures"
	"fmt"
	"log"
	"math"
	"runtime"
	"strconv"
	"unsafe"

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
)

func init() {
	runtime.LockOSThread()
}

func main() {
	if err := glfw.Init(); err != nil {
		log.Fatalln("Could not init glfw:", err)
	}
	defer glfw.Terminate()

	glfw.WindowHint(glfw.ContextVersionMajor, 3)
	glfw.WindowHint(glfw.ContextVersionMinor, 3)
	glfw.WindowHint(glfw.OpenGLProfile, glfw.OpenGLCoreProfile)

	window, err := glfw.CreateWindow(g_width, g_height, "3D-Engine", nil, nil)
	if err != nil {
		log.Fatalln("Could not create a window:", err)
	}
	defer window.Destroy()

	window.MakeContextCurrent()

	if err := gl.Init(); err != nil {
		log.Fatalln("Failed to initialize OpenGL:", err)
	}

	gl.Viewport(0, 0, int32(g_width), int32(g_height))

	gl.Enable(gl.DEPTH_TEST)

	window.SetFramebufferSizeCallback(framebufferSizeCallback)

	window.SetInputMode(glfw.CursorMode, glfw.CursorDisabled)

	lightingShader, err := shaders.CreateShaderProgram("lighting.vert", "lighting.frag")
	if err != nil {
		log.Fatalln("Could not create cube shader:", err)
	}
	defer lightingShader.Delete()

	lightSourceShader, err := shaders.CreateShaderProgram("lighting.vert", "lightSource.frag")
	if err != nil {
		log.Fatalln("Could not create light shader:", err)
	}
	defer lightSourceShader.Delete()

	vertices := loader.LoadModel()

	cubePositions := []mgl32.Vec3{
		{0.0, 0.0, 0.0},
		{2.0, 5.0, -15.0},
		{-1.5, -2.2, -2.5},
		{-3.8, -2.0, -12.3},
		{2.4, -0.4, -3.5},
		{-1.7, 3.0, -7.5},
		{1.3, -2.0, -2.5},
		{1.5, 2.0, -2.5},
		{1.5, 0.2, -1.5},
		{-1.3, 1.0, -1.5},
	}

	pointLightPositions := []mgl32.Vec3{
		{0.7, 0.2, 2.0},
		{2.3, -3.3, -4.0},
		{-4.0, 2.0, -12.0},
		{0.0, 0.0, -3.0},
	}

	var vbo uint32
	gl.GenBuffers(1, &vbo)
	defer gl.DeleteBuffers(1, &vbo)

	gl.BindBuffer(gl.ARRAY_BUFFER, vbo)
	gl.BufferData(gl.ARRAY_BUFFER, len(vertices)*int(sizeof[float32]()), gl.Ptr(vertices), gl.STATIC_DRAW)

	var cubeVao, lightVao uint32
	gl.GenVertexArrays(1, &cubeVao)
	gl.GenVertexArrays(1, &lightVao)
	defer gl.DeleteVertexArrays(1, &cubeVao)
	defer gl.DeleteVertexArrays(1, &lightVao)

	// Cube
	gl.BindVertexArray(cubeVao)
	gl.VertexAttribPointer(0, 3, gl.FLOAT, false, 8*int32(sizeof[float32]()), nil)
	gl.EnableVertexAttribArray(0)

	gl.VertexAttribPointer(1, 3, gl.FLOAT, false, 8*int32(sizeof[float32]()), gl.Ptr(3*sizeof[float32]()))
	gl.EnableVertexAttribArray(1)

	gl.VertexAttribPointer(2, 2, gl.FLOAT, false, 8*int32(sizeof[float32]()), gl.Ptr(6*sizeof[float32]()))
	gl.EnableVertexAttribArray(2)

	// Light
	gl.BindVertexArray(lightVao)
	gl.VertexAttribPointer(0, 3, gl.FLOAT, false, 8*int32(sizeof[float32]()), nil)
	gl.EnableVertexAttribArray(0)

	diffuseMap := textures.Load("container2.jpg")
	specularMap := textures.Load("container2_specular.jpg")
	emissionMap := textures.Load("black.jpg")

	cam := camera.NewCamera()

	window.SetCursorPosCallback(cam.MouseCallback)
	window.SetScrollCallback(cam.ScrollCallback)

	// Wireframe
	// gl.PolygonMode(gl.FRONT_AND_BACK, gl.LINE)

	for !window.ShouldClose() {
		currentFrame := float32(glfw.GetTime())
		deltaTime = currentFrame - lastFrame
		lastFrame = currentFrame

		fpsCounter(window)

		processInput(window, cam)

		gl.ClearColor(0.0, 0.0, 0.0, 1.0)
		gl.Clear(gl.COLOR_BUFFER_BIT | gl.DEPTH_BUFFER_BIT)

		// Lighting
		lightingShader.Use()
		lightingShader.SetVec3Val("viewPos", cam.CameraPos)

		projection := cam.ComputeProjection(g_width, g_height)
		lightingShader.SetMat4("projection", projection)
		view := cam.ComputeView()
		lightingShader.SetMat4("view", view)

		// Directional light
		lightingShader.SetVec3("dirLight.direction", -0.2, -1.0, -0.3)
		lightingShader.SetVec3("dirLight.ambient", 0.2, 0.2, 0.2)
		lightingShader.SetVec3("dirLight.diffuse", 0.5, 0.5, 0.5)
		lightingShader.SetVec3("dirLight.specular", 1.0, 1.0, 1.0)

		// Point light
		for i, pointLightPos := range pointLightPositions {
			lightingShader.SetVec3Val(fmt.Sprintf("pointLights[%d].position", i), pointLightPos)
			lightingShader.SetVec3(fmt.Sprintf("pointLights[%d].ambiant", i), 0.05, 0.05, 0.05)
			lightingShader.SetVec3(fmt.Sprintf("pointLights[%d].diffuse", i), 0.8, 0.8, 0.8)
			lightingShader.SetVec3(fmt.Sprintf("pointLights[%d].specular", i), 1.0, 1.0, 1.0)
			lightingShader.SetFloat(fmt.Sprintf("pointLights[%d].constant", i), 1.0)
			lightingShader.SetFloat(fmt.Sprintf("pointLights[%d].linear", i), 0.09)
			lightingShader.SetFloat(fmt.Sprintf("pointLights[%d].quadratic", i), 0.032)
		}

		// Spot light
		lightingShader.SetVec3Val("spotLight.position", cam.CameraPos)
		lightingShader.SetVec3Val("spotLight.direction", cam.CameraFront)
		lightingShader.SetVec3("spotLight.ambient", 0.0, 0.0, 0.0)
		lightingShader.SetVec3("spotLight.diffuse", 1.0, 1.0, 1.0)
		lightingShader.SetVec3("spotLight.specular", 1.0, 1.0, 1.0)
		lightingShader.SetFloat("spotLight.constant", 1.0)
		lightingShader.SetFloat("spotLight.linear", 0.09)
		lightingShader.SetFloat("spotLight.quadratic", 0.032)
		lightingShader.SetFloat("spotLight.cutOff", float32(math.Cos(float64(mgl32.DegToRad(12.5)))))
		lightingShader.SetFloat("spotLight.outerCutOff", float32(math.Cos(float64(mgl32.DegToRad(15.0)))))

		// Cube
		gl.BindVertexArray(cubeVao)
		for i, cubePos := range cubePositions {
			model := mgl32.Ident4()

			model = model.Mul4(mgl32.Translate3D(cubePos.X(), cubePos.Y(), cubePos.Z()))
			var angle float32 = float32(20.0 * i)
			model = model.Mul4(mgl32.HomogRotate3D(angle, mgl32.Vec3{1.0, 0.3, 0.5}.Normalize()))

			lightingShader.SetMat4("model", model)
			lightingShader.SetInt("material.diffuse", 0)
			lightingShader.SetInt("material.specular", 1)
			lightingShader.SetInt("material.emission", 2)
			lightingShader.SetFloat("material.shininess", 32.0)
			gl.ActiveTexture(gl.TEXTURE0)
			gl.BindTexture(gl.TEXTURE_2D, diffuseMap)
			gl.ActiveTexture(gl.TEXTURE1)
			gl.BindTexture(gl.TEXTURE_2D, specularMap)
			gl.ActiveTexture(gl.TEXTURE2)
			gl.BindTexture(gl.TEXTURE_2D, emissionMap)
			gl.DrawArrays(gl.TRIANGLES, 0, 36)
		}

		// Small light cubes
		gl.BindVertexArray(lightVao)
		for _, pointLightPos := range pointLightPositions {
			lightSourceShader.Use()
			lightSourceShader.SetMat4("projection", projection)
			lightSourceShader.SetMat4("view", view)

			light := mgl32.Ident4()
			light = light.Mul4(mgl32.Translate3D(pointLightPos.X(), pointLightPos.Y(), pointLightPos.Z()))
			light = light.Mul4(mgl32.Scale3D(0.2, 0.2, 0.2))
			lightSourceShader.SetMat4("model", light)
			gl.DrawArrays(gl.TRIANGLES, 0, 36)
		}

		window.SwapBuffers()
		glfw.PollEvents()
	}
}

func processInput(window *glfw.Window, cam *camera.Camera) {
	if window.GetKey(glfw.KeyEscape) == glfw.Press {
		window.SetShouldClose(true)
	}

	cam.ProcessMovement(window, deltaTime)
}

func framebufferSizeCallback(window *glfw.Window, width, height int) {
	g_width = width
	g_height = height

	gl.Viewport(0, 0, int32(width), int32(height))
}

func sizeof[T any]() uintptr {
	var v T
	return unsafe.Sizeof(v)
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
