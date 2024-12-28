package object

import (
	"3d-engine/shaders"
	tex "3d-engine/textures"
	"3d-engine/utils"
	"log"
	"path/filepath"

	"github.com/bloeys/assimp-go/asig"
	"github.com/go-gl/mathgl/mgl32"
)

type Model struct {
	Meshes         []Mesh
	Directory      string
	TexturesLoaded []Texture
}

func (m *Model) Draw(shader *shaders.Shader) {
	for _, mesh := range m.Meshes {
		mesh.Draw(shader)
	}
}

func (m *Model) LoadScene(path string) {
	if utils.GetContext().Debug {
		log.Default().Println("Importing file: ", path)
	}
	scene, release, err := asig.ImportFile(path, asig.PostProcessTriangulate|asig.PostProcessJoinIdenticalVertices|asig.PostProcessOptimizeMeshes|asig.PostProcessFlipUVs|asig.PostProcessSplitLargeMeshes|asig.PostProcessGenNormals)
	if err != nil {
		panic(err)
	}
	defer release()

	m.Directory = filepath.Dir(path)

	m.processNode(scene.RootNode, scene)
}

func (m *Model) processNode(node *asig.Node, scene *asig.Scene) {
	if utils.GetContext().Debug {
		log.Default().Println("Processing node: ", node.Name)
	}
	for i := 0; i < len(node.MeshIndicies); i++ {
		mesh := scene.Meshes[node.MeshIndicies[i]]
		m.Meshes = append(m.Meshes, *m.processMesh(mesh, scene))
	}

	for i := 0; i < len(node.Children); i++ {
		m.processNode(node.Children[i], scene)
	}
}

func (m *Model) processMesh(mesh *asig.Mesh, scene *asig.Scene) *Mesh {
	var vertices []Vertex
	var indices []uint32
	var textures []Texture

	for i := 0; i < len(mesh.Vertices); i++ {
		var vertex Vertex

		vertex.Position = mesh.Vertices[i].Data

		if len(mesh.Normals) > 0 {
			vertex.Normal = mesh.Normals[i].Data
		}

		if len(mesh.TexCoords) > 0 && len(mesh.TexCoords[0]) > i {
			vertex.TexCoords = mgl32.Vec2{mesh.TexCoords[0][i].X(), mesh.TexCoords[0][i].Y()}
		} else {
			vertex.TexCoords = mgl32.Vec2{0.0, 0.0}
		}

		vertices = append(vertices, vertex)
	}

	for _, face := range mesh.Faces {
		for _, indice := range face.Indices {
			indices = append(indices, uint32(indice))
		}
	}

	if mesh.MaterialIndex >= 0 {
		material := scene.Materials[mesh.MaterialIndex]

		var diffuseMaps []Texture = m.loadMaterialTextures(material, asig.TextureTypeDiffuse, "texture_diffuse")
		textures = append(textures, diffuseMaps...)

		var specularMaps []Texture = m.loadMaterialTextures(material, asig.TextureTypeSpecular, "texture_specular")
		textures = append(textures, specularMaps...)

		var normalMaps []Texture = m.loadMaterialTextures(material, asig.TextureTypeNormal, "texture_normal")
		textures = append(textures, normalMaps...)

		var heightMaps []Texture = m.loadMaterialTextures(material, asig.TextureTypeHeight, "texture_height")
		textures = append(textures, heightMaps...)
	}

	return CreateMesh(vertices, indices, textures)
}

func (m *Model) loadMaterialTextures(material *asig.Material, textureType asig.TextureType, typeName string) []Texture {
	var textures []Texture

	for i := 0; i < asig.GetMaterialTextureCount(material, textureType); i++ {
		aTexture, err := asig.GetMaterialTexture(material, textureType, uint(i))
		if err != nil {
			utils.Logger().Println("Error: ", err) // TODO: handle error correctly
			return nil
		}
		skip := false

		for _, loaded := range m.TexturesLoaded {
			if loaded.Path == aTexture.Path {
				textures = append(textures, loaded)
				skip = true
				break
			}
		}

		if !skip {
			var texture Texture

			textureId, err := textureFromFile(aTexture.Path, m.Directory)
			if err != nil {
				panic(err) // TODO: handle better
			}

			texture.Id = textureId
			texture.Path = aTexture.Path
			texture.Type = typeName

			textures = append(textures, texture)
			m.TexturesLoaded = append(m.TexturesLoaded, texture)
		}
	}

	return textures
}

func textureFromFile(path, directory string) (uint32, error) {
	return tex.Load(directory + "/" + path)
}
