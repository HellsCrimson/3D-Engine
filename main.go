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

	shader, err := shaders.CreateShaderProgram()
	if err != nil {
		log.Fatalln("Could not create a shader program:", err)
	}
	defer shader.Delete()

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

	// indices := []uint32{
	// 	0, 1, 3,
	// 	1, 2, 3,
	// }

	var vao, vbo, ebo uint32

	gl.GenVertexArrays(1, &vao)
	gl.GenBuffers(1, &vbo)
	gl.GenBuffers(1, &ebo)
	defer gl.DeleteVertexArrays(1, &vao)
	defer gl.DeleteBuffers(1, &vbo)
	defer gl.DeleteBuffers(1, &ebo)

	gl.BindVertexArray(vao)

	// vbo
	gl.BindBuffer(gl.ARRAY_BUFFER, vbo)
	gl.BufferData(gl.ARRAY_BUFFER, len(vertices)*int(sizeof[float32]()), gl.Ptr(vertices), gl.STATIC_DRAW)

	// ebo
	// gl.BindBuffer(gl.ELEMENT_ARRAY_BUFFER, ebo)
	// gl.BufferData(gl.ELEMENT_ARRAY_BUFFER, len(indices)*int(sizeof[uint32]()), gl.Ptr(indices), gl.STATIC_DRAW)

	// position
	gl.VertexAttribPointer(0, 3, gl.FLOAT, false, 5*int32(sizeof[float32]()), nil)
	gl.EnableVertexAttribArray(0)

	// texture
	gl.VertexAttribPointer(1, 2, gl.FLOAT, false, 5*int32(sizeof[float32]()), gl.Ptr(3*sizeof[float32]()))
	gl.EnableVertexAttribArray(1)

	// unbind
	gl.BindBuffer(gl.ARRAY_BUFFER, 0)
	gl.BindVertexArray(0)
	// End Test Rectangle

	// Texture
	var textureId uint32
	gl.GenTextures(1, &textureId)
	gl.BindTexture(gl.TEXTURE_2D, textureId)

	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_S, gl.REPEAT)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_T, gl.REPEAT)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, gl.LINEAR)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, gl.LINEAR)

	texture := textures.Load("missing")

	gl.TexImage2D(gl.TEXTURE_2D, 0, gl.RGBA, texture.Width, texture.Height, 0, gl.RGBA, gl.UNSIGNED_BYTE, gl.Ptr(texture.Data))
	gl.GenerateMipmap(gl.TEXTURE_2D)

	gl.BindTexture(gl.TEXTURE_2D, 0)
	// End Texture

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

		gl.ClearColor(0.2, 0.3, 0.3, 1.0)
		gl.Clear(gl.COLOR_BUFFER_BIT | gl.DEPTH_BUFFER_BIT)

		gl.ActiveTexture(gl.TEXTURE0)
		gl.BindTexture(gl.TEXTURE_2D, textureId)

		shader.Use()

		projection := cam.ComputeProjection(g_width, g_height)
		shader.SetMat4("projection", projection)

		view := cam.ComputeView()
		shader.SetMat4("view", view)

		gl.BindVertexArray(vao)
		// gl.BindBuffer(gl.ELEMENT_ARRAY_BUFFER, ebo)
		// gl.DrawElements(gl.TRIANGLES, int32(len(indices)), gl.UNSIGNED_INT, nil)
		for i := 0; i < len(cubePositions); i++ {
			model := mgl32.Ident4()
			model = model.Mul4(mgl32.Translate3D(cubePositions[i].X(), cubePositions[i].Y(), cubePositions[i].Z()))
			var angle float32 = float32(20.0 * i)
			model = model.Mul4(mgl32.HomogRotate3D(mgl32.DegToRad(angle), mgl32.Vec3{1.0, 0.3, 0.5}.Normalize()))
			shader.SetMat4("model", model)
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
