package object

import (
	"3d-engine/shaders"
	"3d-engine/utils"

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

	localCenter     mgl32.Vec3
	localBoundsMin  mgl32.Vec3
	localBoundsMax  mgl32.Vec3
	hasLocalBounds  bool
	hasTransparency bool

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
	mesh.computeMetadata()
	mesh.setupMesh()
	return &mesh
}

func (m *Mesh) computeMetadata() {
	if len(m.Vertices) == 0 {
		m.localCenter = mgl32.Vec3{0, 0, 0}
		m.localBoundsMin = mgl32.Vec3{0, 0, 0}
		m.localBoundsMax = mgl32.Vec3{0, 0, 0}
		m.hasLocalBounds = false
	} else {
		sum := mgl32.Vec3{0, 0, 0}
		minBounds := mgl32.Vec3{1e9, 1e9, 1e9}
		maxBounds := mgl32.Vec3{-1e9, -1e9, -1e9}
		for _, v := range m.Vertices {
			sum = sum.Add(v.Position)
			if v.Position.X() < minBounds.X() {
				minBounds[0] = v.Position.X()
			}
			if v.Position.Y() < minBounds.Y() {
				minBounds[1] = v.Position.Y()
			}
			if v.Position.Z() < minBounds.Z() {
				minBounds[2] = v.Position.Z()
			}
			if v.Position.X() > maxBounds.X() {
				maxBounds[0] = v.Position.X()
			}
			if v.Position.Y() > maxBounds.Y() {
				maxBounds[1] = v.Position.Y()
			}
			if v.Position.Z() > maxBounds.Z() {
				maxBounds[2] = v.Position.Z()
			}
		}
		inv := float32(1.0 / float32(len(m.Vertices)))
		m.localCenter = sum.Mul(inv)
		m.localBoundsMin = minBounds
		m.localBoundsMax = maxBounds
		m.hasLocalBounds = true
	}

	for _, tex := range m.Textures {
		if tex.HasTransparency {
			m.hasTransparency = true
			break
		}
	}
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
	m.DrawPass(shader, m.hasTransparency)
}

func (m *Mesh) IsTransparent() bool {
	return m.hasTransparency
}

func (m *Mesh) WorldCenter(modelMat mgl32.Mat4) mgl32.Vec3 {
	world := modelMat.Mul4x1(mgl32.Vec4{m.localCenter.X(), m.localCenter.Y(), m.localCenter.Z(), 1.0})
	return world.Vec3()
}

func (m *Mesh) WorldAABB(modelMat mgl32.Mat4) (mgl32.Vec3, mgl32.Vec3) {
	if !m.hasLocalBounds {
		center := m.WorldCenter(modelMat)
		return center, center
	}

	minB := m.localBoundsMin
	maxB := m.localBoundsMax
	corners := [8]mgl32.Vec3{
		{minB.X(), minB.Y(), minB.Z()},
		{maxB.X(), minB.Y(), minB.Z()},
		{minB.X(), maxB.Y(), minB.Z()},
		{maxB.X(), maxB.Y(), minB.Z()},
		{minB.X(), minB.Y(), maxB.Z()},
		{maxB.X(), minB.Y(), maxB.Z()},
		{minB.X(), maxB.Y(), maxB.Z()},
		{maxB.X(), maxB.Y(), maxB.Z()},
	}

	first := modelMat.Mul4x1(mgl32.Vec4{corners[0].X(), corners[0].Y(), corners[0].Z(), 1.0}).Vec3()
	worldMin := first
	worldMax := first
	for i := 1; i < len(corners); i++ {
		worldCorner := modelMat.Mul4x1(mgl32.Vec4{corners[i].X(), corners[i].Y(), corners[i].Z(), 1.0}).Vec3()
		if worldCorner.X() < worldMin.X() {
			worldMin[0] = worldCorner.X()
		}
		if worldCorner.Y() < worldMin.Y() {
			worldMin[1] = worldCorner.Y()
		}
		if worldCorner.Z() < worldMin.Z() {
			worldMin[2] = worldCorner.Z()
		}
		if worldCorner.X() > worldMax.X() {
			worldMax[0] = worldCorner.X()
		}
		if worldCorner.Y() > worldMax.Y() {
			worldMax[1] = worldCorner.Y()
		}
		if worldCorner.Z() > worldMax.Z() {
			worldMax[2] = worldCorner.Z()
		}
	}

	return worldMin, worldMax
}

func (m *Mesh) DrawPass(shader *shaders.Shader, drawTransparent bool) {
	if drawTransparent != m.hasTransparency {
		return
	}

	var diffuseNr, specularNr, normalNr, heightNr uint32 = 1, 1, 1, 1

	shader.SetBool("material.has_diffuse", false)
	shader.SetBool("material.has_specular", false)
	shader.SetBool("material.has_emission", false)
	shader.SetBool("material.has_reflection", false)

	i := int32(0)
	for ; i < int32(len(m.Textures)); i++ {
		gl.ActiveTexture(gl.TEXTURE0 + uint32(i))

		name := m.Textures[i].Type

		if name == "texture_diffuse" {
			diffuseNr++
			shader.SetBool("material.has_diffuse", true)
		} else if name == "texture_specular" {
			specularNr++
			shader.SetBool("material.has_specular", true)
		} else if name == "texture_normal" {
			normalNr++
		} else if name == "texture_height" {
			heightNr++
		} else {
			utils.Logger().Println("Unsupported texture type: ", name)
		}

		shader.SetInt("material."+name, i)
		shader.SetFloat("material.shininess", 32)
		gl.BindTexture(gl.TEXTURE_2D, m.Textures[i].Id)
	}

	gl.ActiveTexture(gl.TEXTURE0 + uint32(i))
	shader.SetInt("material.missing_texture", i)
	gl.BindTexture(gl.TEXTURE_2D, shader.NoTexture)

	if drawTransparent {
		gl.Enable(gl.BLEND)
		gl.BlendFunc(gl.SRC_ALPHA, gl.ONE_MINUS_SRC_ALPHA)
		gl.BlendEquation(gl.FUNC_ADD)
		// gl.DepthMask(false)
	}

	gl.BindVertexArray(m.vao)
	gl.DrawElements(gl.TRIANGLES, int32(len(m.Indices)), gl.UNSIGNED_INT, nil)

	gl.BindVertexArray(0)
	gl.ActiveTexture(gl.TEXTURE0)

	if drawTransparent {
		gl.Disable(gl.BLEND)
		// gl.DepthMask(true)
	}
}
