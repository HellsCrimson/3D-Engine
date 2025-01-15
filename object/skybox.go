package object

import (
	"3d-engine/shaders"
	"3d-engine/textures"
	"3d-engine/utils"

	"github.com/go-gl/gl/v4.6-core/gl"
	"github.com/go-gl/mathgl/mgl32"
)

type Skybox struct {
	Path              string
	SkyboxTextureUnit uint32

	Shader    *shaders.Shader
	TextureId uint32

	vao uint32
	vbo uint32

	skyboxVertices []float32
}

func CreateSkybox(path string) *Skybox {
	return &Skybox{
		Path:              path,
		SkyboxTextureUnit: 11,
	}
}

func (s *Skybox) LoadCubemap() {
	var err error
	s.TextureId, err = textures.LoadCubemap(s.Path)
	if err != nil {
		utils.Logger().Println("Error loading cubemap: ", err)
	}

	s.skyboxVertices = getSkyboxCube()

	gl.GenVertexArrays(1, &s.vao)
	gl.GenBuffers(1, &s.vbo)
	gl.BindVertexArray(s.vao)
	gl.BindBuffer(gl.ARRAY_BUFFER, s.vbo)
	gl.BufferData(gl.ARRAY_BUFFER, len(s.skyboxVertices)*int(utils.Sizeof[float32]()), gl.Ptr(s.skyboxVertices), gl.STATIC_DRAW)
	gl.EnableVertexAttribArray(0)
	gl.VertexAttribPointer(0, 3, gl.FLOAT, false, 3*int32(utils.Sizeof[float32]()), nil)
}

func (s *Skybox) RenderSkybox(view mgl32.Mat4, proj mgl32.Mat4) {
	gl.DepthFunc(gl.LEQUAL)
	s.Shader.Use()
	s.Shader.SetMat4("view", view)
	s.Shader.SetMat4("projection", proj)

	gl.BindVertexArray(s.vao)
	gl.ActiveTexture(gl.TEXTURE0 + s.SkyboxTextureUnit)
	gl.BindTexture(gl.TEXTURE_CUBE_MAP, s.TextureId)
	gl.DrawArrays(gl.TRIANGLES, 0, int32(len(s.skyboxVertices)))
	gl.BindVertexArray(0)
	gl.DepthFunc(gl.LESS)
}

func getSkyboxCube() []float32 {
	return []float32{
		-1.0, 1.0, -1.0,
		-1.0, -1.0, -1.0,
		1.0, -1.0, -1.0,
		1.0, -1.0, -1.0,
		1.0, 1.0, -1.0,
		-1.0, 1.0, -1.0,

		-1.0, -1.0, 1.0,
		-1.0, -1.0, -1.0,
		-1.0, 1.0, -1.0,
		-1.0, 1.0, -1.0,
		-1.0, 1.0, 1.0,
		-1.0, -1.0, 1.0,

		1.0, -1.0, -1.0,
		1.0, -1.0, 1.0,
		1.0, 1.0, 1.0,
		1.0, 1.0, 1.0,
		1.0, 1.0, -1.0,
		1.0, -1.0, -1.0,

		-1.0, -1.0, 1.0,
		-1.0, 1.0, 1.0,
		1.0, 1.0, 1.0,
		1.0, 1.0, 1.0,
		1.0, -1.0, 1.0,
		-1.0, -1.0, 1.0,

		-1.0, 1.0, -1.0,
		1.0, 1.0, -1.0,
		1.0, 1.0, 1.0,
		1.0, 1.0, 1.0,
		-1.0, 1.0, 1.0,
		-1.0, 1.0, -1.0,

		-1.0, -1.0, -1.0,
		-1.0, -1.0, 1.0,
		1.0, -1.0, -1.0,
		1.0, -1.0, -1.0,
		-1.0, -1.0, 1.0,
		1.0, -1.0, 1.0,
	}
}
