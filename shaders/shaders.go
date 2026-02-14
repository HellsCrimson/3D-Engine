package shaders

import (
	"3d-engine/utils"
	"embed"
	"fmt"
	"strings"

	"github.com/go-gl/gl/v4.6-core/gl"
)

//go:embed *.vert *.frag
var shaderDir embed.FS

type Shader struct {
	ProgramId uint32
	NoTexture uint32
}

func (s *Shader) Use() {
	gl.UseProgram(s.ProgramId)
}

func (s *Shader) Delete() {
	gl.DeleteProgram(s.ProgramId)
}

func CreateShaderProgram(nameVertex, nameFragment string) (*Shader, error) {
	vertexShader, err := compileShader(nameVertex, gl.VERTEX_SHADER)
	if err != nil {
		return nil, err
	}
	fragmentShader, err := compileShader(nameFragment, gl.FRAGMENT_SHADER)
	if err != nil {
		gl.DeleteShader(vertexShader)
		return nil, err
	}

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

func compileShader(name string, shaderType uint32) (uint32, error) {
	shaderSourceStr, err := getShader(name)
	if err != nil {
		return 0, err
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
		gl.DeleteShader(shader)
		return 0, fmt.Errorf("ERROR::SHADER::%s::COMPILATION_FAILED %s", strings.ToUpper(name), string(infoLog))
	}

	return shader, nil
}

func getShader(name string) (string, error) {
	content, err := shaderDir.ReadFile(name)
	if err != nil {
		return "", utils.Logger().Errorf("failed to read file: %s\n", err)
	}

	return string(content), nil
}
