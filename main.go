package main

import (
	"log"
	"unsafe"

	"github.com/go-gl/gl/v4.6-core/gl"
	"github.com/go-gl/glfw/v3.3/glfw"
)

var (
	g_width         = 800
	g_height        = 600
	g_shaderProgram uint32
)

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

	window.SetFramebufferSizeCallback(framebuffer_size_callback)

	g_shaderProgram, err = createShaderProgram()
	if err != nil {
		log.Fatalln("Could not create a shader program:", err)
	}
	defer gl.DeleteProgram(g_shaderProgram)

	// Test Rectangle
	vertices := []float32{
		0.5, 0.5, 0.0, // top right
		0.5, -0.5, 0.0, // bottom right
		-0.5, -0.5, 0.0, // bottom left
		-0.5, 0.5, 0.0, // top left
	}

	indices := []uint32{
		0, 1, 3,
		1, 2, 3,
	}

	var vao, vbo, ebo uint32
	gl.GenVertexArrays(1, &vao)
	gl.GenBuffers(1, &vbo)
	gl.GenBuffers(1, &ebo)
	defer gl.DeleteVertexArrays(1, &vao)
	defer gl.DeleteBuffers(1, &vbo)
	defer gl.DeleteBuffers(1, &ebo)

	gl.BindVertexArray(vao)

	gl.BindBuffer(gl.ARRAY_BUFFER, vbo)
	gl.BufferData(gl.ARRAY_BUFFER, len(vertices)*int(getSize[float32]()), gl.Ptr(vertices), gl.STATIC_DRAW)

	gl.BindBuffer(gl.ELEMENT_ARRAY_BUFFER, ebo)
	gl.BufferData(gl.ELEMENT_ARRAY_BUFFER, len(indices)*int(getSize[uint32]()), gl.Ptr(indices), gl.STATIC_DRAW)

	gl.VertexAttribPointer(0, 3, gl.FLOAT, false, 3*int32(getSize[float32]()), nil)
	gl.EnableVertexAttribArray(0)

	gl.BindBuffer(gl.ARRAY_BUFFER, 0)
	gl.BindVertexArray(0)
	// End Test Rectangle

	// Wireframe
	// gl.PolygonMode(gl.FRONT_AND_BACK, gl.LINE)

	for !window.ShouldClose() {
		processInput(window)

		gl.ClearColor(0.2, 0.3, 0.3, 1.0)
		gl.Clear(gl.COLOR_BUFFER_BIT)

		gl.UseProgram(g_shaderProgram)

		gl.BindVertexArray(vao)
		gl.BindBuffer(gl.ELEMENT_ARRAY_BUFFER, ebo)
		gl.DrawElements(gl.TRIANGLES, int32(len(indices)), gl.UNSIGNED_INT, nil)

		window.SwapBuffers()
		glfw.PollEvents()
	}
}

func processInput(window *glfw.Window) {
	if window.GetKey(glfw.KeyEscape) == glfw.Press {
		window.SetShouldClose(true)
	}
}

func framebuffer_size_callback(window *glfw.Window, width, height int) {
	g_width = width
	g_height = height

	gl.Viewport(0, 0, int32(width), int32(height))
}

func getSize[T any]() uintptr {
	var v T
	return unsafe.Sizeof(v)
}
