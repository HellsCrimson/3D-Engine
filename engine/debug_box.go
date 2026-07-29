package engine

import (
	"3d-engine/shaders"

	"github.com/go-gl/gl/v4.6-core/gl"
	"github.com/go-gl/mathgl/mgl32"
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
