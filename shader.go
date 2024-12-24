package main

import (
	"fmt"
	"io"
	"log"
	"os"
	"strings"

	"github.com/go-gl/gl/v4.6-core/gl"
)

func createShaderProgram() (uint32, error) {
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
		return 0, fmt.Errorf("ERROR::SHADER::PROGRAM::COMPILATION_FAILED\n%s\n", string(infoLog))
	}

	return shaderProgram, nil
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
	file, err := os.Open("shaders/" + name + ".glsl")
	if err != nil {
		return "", fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	content, err := io.ReadAll(file)
	if err != nil {
		return "", fmt.Errorf("failed to read file: %w", err)
	}

	return string(content), nil
}
