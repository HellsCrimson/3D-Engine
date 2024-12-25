package shaders

import (
	"github.com/go-gl/gl/v4.6-core/gl"
	"github.com/go-gl/mathgl/mgl32"
)

func (s Shader) SetBool(name string, val bool) {
	var valInt int32
	if val {
		valInt = 1
	}
	gl.Uniform1i(gl.GetUniformLocation(s.ProgramId, gl.Str(name+"\x00")), valInt)
}

func (s Shader) SetInt(name string, val int32) {
	gl.Uniform1i(gl.GetUniformLocation(s.ProgramId, gl.Str(name+"\x00")), val)
}

func (s Shader) SetFloat(name string, val float32) {
	gl.Uniform1f(gl.GetUniformLocation(s.ProgramId, gl.Str(name+"\x00")), val)
}

func (s Shader) SetVec2Val(name string, val mgl32.Vec2) {
	gl.Uniform2fv(gl.GetUniformLocation(s.ProgramId, gl.Str(name+"\x00")), 1, &val[0])
}

func (s Shader) SetVec2(name string, x, y float32) {
	gl.Uniform2f(gl.GetUniformLocation(s.ProgramId, gl.Str(name+"\x00")), x, y)
}

func (s Shader) SetVec3Val(name string, val mgl32.Vec3) {
	gl.Uniform3fv(gl.GetUniformLocation(s.ProgramId, gl.Str(name+"\x00")), 1, &val[0])
}

func (s Shader) SetVec3(name string, x, y, z float32) {
	gl.Uniform3f(gl.GetUniformLocation(s.ProgramId, gl.Str(name+"\x00")), x, y, z)
}

func (s Shader) SetVec4Val(name string, val mgl32.Vec4) {
	gl.Uniform4fv(gl.GetUniformLocation(s.ProgramId, gl.Str(name+"\x00")), 1, &val[0])
}

func (s Shader) SetVec4(name string, x, y, z, w float32) {
	gl.Uniform4f(gl.GetUniformLocation(s.ProgramId, gl.Str(name+"\x00")), x, y, z, w)
}

func (s Shader) SetMat2(name string, val mgl32.Mat2) {
	gl.UniformMatrix2fv(gl.GetUniformLocation(s.ProgramId, gl.Str(name+"\x00")), 1, false, &val[0])
}

func (s Shader) SetMat3(name string, val mgl32.Mat3) {
	gl.UniformMatrix3fv(gl.GetUniformLocation(s.ProgramId, gl.Str(name+"\x00")), 1, false, &val[0])
}

func (s Shader) SetMat4(name string, val mgl32.Mat4) {
	gl.UniformMatrix4fv(gl.GetUniformLocation(s.ProgramId, gl.Str(name+"\x00")), 1, false, &val[0])
}
