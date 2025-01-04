package object

import (
	"3d-engine/shaders"
	"3d-engine/utils"
	"strconv"

	"github.com/go-gl/gl/v4.6-core/gl"
	"github.com/go-gl/mathgl/mgl32"
)

type Vertex struct {
	Position  mgl32.Vec3
	Normal    mgl32.Vec3
	TexCoords mgl32.Vec2
}

type Texture struct {
	Id              uint32
	Type            string
	Path            string
	HasTransparency bool
}

type Mesh struct {
	Vertices []Vertex
	Indices  []uint32
	Textures []Texture

	vao uint32
	vbo uint32
	ebo uint32
}

func CreateMesh(vertices []Vertex, indices []uint32, textures []Texture) *Mesh {
	mesh := Mesh{
		Vertices: vertices,
		Indices:  indices,
		Textures: textures,
	}
	mesh.setupMesh()
	return &mesh
}

func (m *Mesh) setupMesh() {
	gl.GenVertexArrays(1, &m.vao)
	gl.GenBuffers(1, &m.vbo)
	gl.GenBuffers(1, &m.ebo)

	gl.BindVertexArray(m.vao)
	gl.BindBuffer(gl.ARRAY_BUFFER, m.vbo)

	gl.BufferData(gl.ARRAY_BUFFER, len(m.Vertices)*int(utils.Sizeof[Vertex]()), gl.Ptr(m.Vertices), gl.STATIC_DRAW)

	gl.BindBuffer(gl.ELEMENT_ARRAY_BUFFER, m.ebo)
	gl.BufferData(gl.ELEMENT_ARRAY_BUFFER, len(m.Indices)*int(utils.Sizeof[uint32]()), gl.Ptr(&m.Indices[0]), gl.STATIC_DRAW)

	gl.EnableVertexAttribArray(0)
	gl.VertexAttribPointer(0, 3, gl.FLOAT, false, int32(utils.Sizeof[Vertex]()), nil)

	gl.EnableVertexAttribArray(1)
	offset, _ := utils.OffsetOf[Vertex]("Normal")
	// if err != nil {
	// 	fmt.Errorf("Could not get offset: %s\n", err)
	// 	return
	// }
	gl.VertexAttribPointer(1, 3, gl.FLOAT, false, int32(utils.Sizeof[Vertex]()), gl.Ptr(offset))

	gl.EnableVertexAttribArray(2)
	offset, _ = utils.OffsetOf[Vertex]("TexCoords")
	// if err != nil {
	// 	fmt.Errorf("Could not get offset: %s\n", err)
	// 	return
	// }
	gl.VertexAttribPointer(2, 2, gl.FLOAT, false, int32(utils.Sizeof[Vertex]()), gl.Ptr(offset))

	gl.BindVertexArray(0)
}

func (m *Mesh) Draw(shader *shaders.Shader) {
	m.DrawSpecific(shader, false)
	m.DrawSpecific(shader, true)
}

func (m *Mesh) DrawSpecific(shader *shaders.Shader, drawTransparent bool) {
	var diffuseNr, specularNr, normalNr, heightNr uint32 = 1, 1, 1, 1

	shouldDraw := len(m.Textures) == 0 && !drawTransparent
	i := int32(0)
	for ; i < int32(len(m.Textures)); i++ {
		if m.Textures[i].HasTransparency == drawTransparent {
			shouldDraw = true
		}

		gl.ActiveTexture(gl.TEXTURE0 + uint32(i))

		var number string
		name := m.Textures[i].Type

		if name == "texture_diffuse" {
			number = strconv.FormatUint(uint64(diffuseNr), 10)
			diffuseNr++
			shader.SetBool("material.has_diffuse", true)
		} else if name == "texture_specular" {
			number = strconv.FormatUint(uint64(specularNr), 10)
			specularNr++
		} else if name == "texture_normal" {
			number = strconv.FormatUint(uint64(normalNr), 10)
			normalNr++
		} else if name == "texture_height" {
			number = strconv.FormatUint(uint64(heightNr), 10)
			heightNr++
		}

		shader.SetInt("material."+name+number, i)
		gl.BindTexture(gl.TEXTURE_2D, m.Textures[i].Id)
	}

	if !shouldDraw {
		return
	}

	if drawTransparent {
		gl.Enable(gl.BLEND)
		gl.BlendFunc(gl.SRC_ALPHA, gl.ONE_MINUS_SRC_ALPHA)
		gl.BlendEquation(gl.FUNC_ADD)
		// gl.DepthMask(false)
	}

	gl.ActiveTexture(gl.TEXTURE0 + uint32(i))
	shader.SetInt("material.missing_texture", i)
	gl.BindTexture(gl.TEXTURE_2D, shader.NoTexture)

	gl.BindVertexArray(m.vao)
	gl.DrawElements(gl.TRIANGLES, int32(len(m.Indices)), gl.UNSIGNED_INT, nil)

	gl.BindVertexArray(0)
	gl.ActiveTexture(gl.TEXTURE0)

	if drawTransparent {
		gl.Disable(gl.BLEND)
		// gl.DepthMask(true)
	}
}
