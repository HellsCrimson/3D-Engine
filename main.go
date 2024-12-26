package main

import (
	"3d-engine/camera"
	"3d-engine/loader"
	"3d-engine/shaders"
	"3d-engine/textures"
	"log"
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

var (
	lightPos = mgl32.Vec3{1.2, 1.0, 2.0}
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

	texture := textures.Load("container2.jpg")

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
		lightingShader.SetVec3Val("light.position", lightPos)
		lightingShader.SetVec3("light.ambient", 0.2, 0.2, 0.2)
		lightingShader.SetVec3("light.diffuse", 0.5, 0.5, 0.5)
		lightingShader.SetVec3("light.specular", 1.0, 1.0, 1.0)
		lightingShader.SetVec3Val("viewPos", cam.CameraPos)

		projection := cam.ComputeProjection(g_width, g_height)
		lightingShader.SetMat4("projection", projection)
		view := cam.ComputeView()
		lightingShader.SetMat4("view", view)

		// Cube
		model := mgl32.Ident4()
		lightingShader.SetMat4("model", model)
		lightingShader.SetInt("material.diffuse", 0)
		lightingShader.SetVec3("material.specular", 0.5, 0.5, 0.5)
		lightingShader.SetFloat("material.shininess", 32.0)
		gl.ActiveTexture(gl.TEXTURE0)
		gl.BindTexture(gl.TEXTURE_2D, texture)
		gl.BindVertexArray(cubeVao)
		gl.DrawArrays(gl.TRIANGLES, 0, 36)

		// Light source
		lightSourceShader.Use()
		lightSourceShader.SetMat4("projection", projection)
		lightSourceShader.SetMat4("view", view)
		light := mgl32.Ident4()
		light = light.Mul4(mgl32.Translate3D(lightPos.X(), lightPos.Y(), lightPos.Z()))
		light = light.Mul4(mgl32.Scale3D(0.2, 0.2, 0.2))
		lightSourceShader.SetMat4("model", light)
		gl.BindVertexArray(lightVao)
		gl.DrawArrays(gl.TRIANGLES, 0, 36)

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
