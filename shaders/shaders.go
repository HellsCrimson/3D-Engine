package shaders

import (
	"3d-engine/utils"
	"embed"
	"strings"

	"github.com/go-gl/gl/v4.6-core/gl"
)

//go:embed *.vert *.frag
var shaderDir embed.FS

type Shader struct {
	ProgramId uint32
}

func (s *Shader) Use() {
	gl.UseProgram(s.ProgramId)
}

func (s *Shader) Delete() {
	gl.DeleteProgram(s.ProgramId)
}

func CreateShaderProgram(nameVertex, nameFragment string) (*Shader, error) {
	vertexShader := compileShader(nameVertex, gl.VERTEX_SHADER)
	fragmentShader := compileShader(nameFragment, gl.FRAGMENT_SHADER)

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
		return nil, utils.Logger().Errorf("ERROR::SHADER::PROGRAM::COMPILATION_FAILED\n%s\n", string(infoLog))
	}

	return &Shader{ProgramId: shaderProgram}, nil
}

func compileShader(name string, shaderType uint32) uint32 {
	shaderSourceStr, err := getShader(name)
	if err != nil {
		utils.Logger().Fatalln(err)
	}

	shaderSource, freeShader := gl.Strs(shaderSourceStr + "\x00")
	shader := gl.CreateShader(shaderType)
	gl.ShaderSource(shader, 1, shaderSource, nil)
	freeShader()
	gl.CompileShader(shader)

	var success int32
	gl.GetShaderiv(shader, gl.COMPILE_STATUS, &success)

	if success == gl.FALSE {
		infoLog := make([]byte, 512)
		gl.GetShaderInfoLog(shader, 512, nil, &infoLog[0])
		utils.Logger().Fatalln("ERROR::SHADER::"+strings.ToUpper(name)+"::COMPILATION_FAILED", string(infoLog))
	}

	return shader
}

func getShader(name string) (string, error) {
	content, err := shaderDir.ReadFile(name)
	if err != nil {
		return "", utils.Logger().Errorf("failed to read file: %s\n", err)
	}

	return string(content), nil
}
