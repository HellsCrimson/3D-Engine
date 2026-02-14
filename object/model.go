package object

import (
	"3d-engine/shaders"
	tex "3d-engine/textures"
	"3d-engine/utils"
	"fmt"
	"log"
	"path/filepath"

	"github.com/bloeys/assimp-go/asig"
	"github.com/go-gl/mathgl/mgl32"
)

type Model struct {
	Id             uint32
	Name           string
	Meshes         []Mesh
	Directory      string
	TexturesLoaded []Texture

	Coordinates mgl32.Vec3
	Rotation    mgl32.Vec4 // W is for the angle
	Scale       mgl32.Vec3
}

func (m *Model) Draw(shader *shaders.Shader) {
	for _, mesh := range m.Meshes {
		mesh.Draw(shader)
	}
}

func (m *Model) LoadScene(path string) error {
	if utils.GetContext().DebugLevel > utils.NoDebug {
		log.Default().Println("Importing file: ", path)
	}
	scene, release, err := asig.ImportFile(path, asig.PostProcessTriangulate|asig.PostProcessJoinIdenticalVertices|asig.PostProcessOptimizeMeshes|asig.PostProcessFlipUVs|asig.PostProcessSplitLargeMeshes|asig.PostProcessGenNormals)
	if err != nil {
		return fmt.Errorf("failed to import model %q: %w", path, err)
	}
	defer release()

	m.Directory = filepath.Dir(path)

	if err := m.processNode(scene.RootNode, scene); err != nil {
		return err
	}
	return nil
}

func (m *Model) processNode(node *asig.Node, scene *asig.Scene) error {
	if utils.GetContext().DebugLevel > utils.NoDebug {
		log.Default().Println("Processing node: ", node.Name)
	}
	for i := 0; i < len(node.MeshIndicies); i++ {
		mesh := scene.Meshes[node.MeshIndicies[i]]
		processedMesh, err := m.processMesh(mesh, scene)
		if err != nil {
			return err
		}
		m.Meshes = append(m.Meshes, *processedMesh)
	}

	for i := 0; i < len(node.Children); i++ {
		if err := m.processNode(node.Children[i], scene); err != nil {
			return err
		}
	}
	return nil
}

func (m *Model) processMesh(mesh *asig.Mesh, scene *asig.Scene) (*Mesh, error) {
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

		diffuseMaps, err := m.loadMaterialTextures(material, asig.TextureTypeDiffuse, "texture_diffuse")
		if err != nil {
			return nil, err
		}
		textures = append(textures, diffuseMaps...)

		specularMaps, err := m.loadMaterialTextures(material, asig.TextureTypeSpecular, "texture_specular")
		if err != nil {
			return nil, err
		}
		textures = append(textures, specularMaps...)

		normalMaps, err := m.loadMaterialTextures(material, asig.TextureTypeNormal, "texture_normal")
		if err != nil {
			return nil, err
		}
		textures = append(textures, normalMaps...)

		heightMaps, err := m.loadMaterialTextures(material, asig.TextureTypeHeight, "texture_height")
		if err != nil {
			return nil, err
		}
		textures = append(textures, heightMaps...)
	}

	return CreateMesh(vertices, indices, textures), nil
}

func (m *Model) loadMaterialTextures(material *asig.Material, textureType asig.TextureType, typeName string) ([]Texture, error) {
	var textures []Texture

	for i := 0; i < asig.GetMaterialTextureCount(material, textureType); i++ {
		aTexture, err := asig.GetMaterialTexture(material, textureType, uint(i))
		if err != nil {
			return nil, fmt.Errorf("failed to get material texture: %w", err)
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

			isTransparent := false
			textureId, err := textureFromFile(aTexture.Path, m.Directory, &isTransparent)
			if err != nil {
				return nil, fmt.Errorf("failed to load texture %q: %w", aTexture.Path, err)
			}

			texture.Id = textureId
			texture.Path = aTexture.Path
			texture.Type = typeName
			texture.HasTransparency = isTransparent

			textures = append(textures, texture)
			m.TexturesLoaded = append(m.TexturesLoaded, texture)
		}
	}

	return textures, nil
}

func textureFromFile(path, directory string, isTransparent *bool) (uint32, error) {
	return tex.Load(directory+"/"+path, isTransparent)
}
