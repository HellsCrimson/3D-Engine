package shaders

import (
	"embed"
	"fmt"
	"log"
	"strings"

	"github.com/go-gl/gl/v4.6-core/gl"
)

//go:embed *.glsl
var shaderDir embed.FS

type Shader struct {
	ProgramId uint32
}

func (s Shader) Use() {
	gl.UseProgram(s.ProgramId)
}

func (s Shader) SetBool(name string, val bool) {
	var valInt int32
	if val {
		valInt = 1
	}
	strs, freeFunc := gl.Strs(name)
	gl.Uniform1i(gl.GetUniformLocation(s.ProgramId, *strs), valInt)
	freeFunc()
}

func (s Shader) SetInt(name string, val int32) {
	strs, freeFunc := gl.Strs(name)
	gl.Uniform1i(gl.GetUniformLocation(s.ProgramId, *strs), val)
	freeFunc()
}

func (s Shader) SetFloat(name string, val float32) {
	strs, freeFunc := gl.Strs(name)
	gl.Uniform1f(gl.GetUniformLocation(s.ProgramId, *strs), val)
	freeFunc()
}

func (s Shader) Delete() {
	gl.DeleteProgram(s.ProgramId)
}

func CreateShaderProgram() (*Shader, error) {
	vertexShader := compileShader("vertex", gl.VERTEX_SHADER)
	fragmentShader := compileShader("fragment", gl.FRAGMENT_SHADER)

	defer gl.DeleteShader(vertexShader)
	defer gl.DeleteShader(fragmentShader)

	shaderProgram := gl.CreateProgram()
	gl.AttachShader(shaderProgram, vertexShader)
	gl.AttachShader(shaderProgram, fragmentShader)
	gl.LinkProgram(shaderProgram)

	var success int32
	gl.GetProgramiv(shaderProgram, gl.LINK_STATUS, &success)
	if success == gl.FALSE {
		infoLog := make([]byte, 512)
		gl.GetProgramInfoLog(shaderProgram, 512, nil, &infoLog[0])
		return nil, fmt.Errorf("ERROR::SHADER::PROGRAM::COMPILATION_FAILED\n%s\n", string(infoLog))
	}

	return &Shader{ProgramId: shaderProgram}, nil
}

func compileShader(name string, shaderType uint32) uint32 {
	shaderSourceStr, err := getShader(name)
	if err != nil {
		log.Fatalln(err)
	}

	shaderSource, freeShader := gl.Strs(shaderSourceStr)
	shader := gl.CreateShader(shaderType)
	gl.ShaderSource(shader, 1, shaderSource, nil)
	freeShader()
	gl.CompileShader(shader)

	var success int32
	gl.GetShaderiv(shader, gl.COMPILE_STATUS, &success)

	if success == gl.FALSE {
		infoLog := make([]byte, 512)
		gl.GetShaderInfoLog(shader, 512, nil, &infoLog[0])
		fmt.Println("ERROR::SHADER::"+strings.ToUpper(name)+"::COMPILATION_FAILED", string(infoLog))
	}

	return shader
}

func getShader(name string) (string, error) {
	content, err := shaderDir.ReadFile(name + ".glsl")
	if err != nil {
		return "", fmt.Errorf("failed to read file: %w", err)
	}

	return string(content), nil
}
