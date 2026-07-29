package object

import (
	"3d-engine/shaders"
	tex "3d-engine/textures"
	"3d-engine/utils"
	"fmt"
	"path/filepath"

	"github.com/bloeys/assimp-go/asig"
	"github.com/go-gl/mathgl/mgl32"
)

// Model is a renderable asset: geometry, materials and the bounds fitted around
// them. It carries no placement — where it appears in the world is the Entity's
// business — so one Model can back several entities.
type Model struct {
	Path           string
	Meshes         []Mesh
	Directory      string
	TexturesLoaded []Texture

	localBounds    AABB
	hasLocalBounds bool
}

// Delete frees every GPU resource the model owns: the meshes' buffers, and one
// release per texture it acquired. Must run on the GL thread.
func (m *Model) Delete() {
	for i := range m.Meshes {
		m.Meshes[i].Delete()
	}
	m.Meshes = nil

	for _, texture := range m.TexturesLoaded {
		if err := tex.Release(texture.Path); err != nil {
			utils.Logger().Printf("Releasing texture: %v", err)
		}
	}
	m.TexturesLoaded = nil
}

func (m *Model) Draw(shader *shaders.Shader) {
	for _, mesh := range m.Meshes {
		mesh.Draw(shader)
	}
}

// Import loads a model file into GPU-backed meshes. It was called LoadScene,
// which collided confusingly with SceneManager.LoadScene.
func (m *Model) Import(path string) error {
	utils.Logger().Infoln("Importing file: ", path)
	m.Path = path
	scene, release, err := asig.ImportFile(path, asig.PostProcessTriangulate|asig.PostProcessJoinIdenticalVertices|asig.PostProcessOptimizeMeshes|asig.PostProcessFlipUVs|asig.PostProcessSplitLargeMeshes|asig.PostProcessGenNormals)
	if err != nil {
		return fmt.Errorf("failed to import model %q: %w", path, err)
	}
	defer release()

	m.Directory = filepath.Dir(path)

	if err := m.processNode(scene.RootNode, scene); err != nil {
		return err
	}
	m.computeLocalBounds()
	return nil
}

func (m *Model) processNode(node *asig.Node, scene *asig.Scene) error {
	utils.Logger().Infoln("Processing node: ", node.Name)
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
		// Textures are keyed by their resolved path, since that is what has to
		// be handed back to the cache on release.
		resolved := filepath.Join(m.Directory, aTexture.Path)

		skip := false
		for _, loaded := range m.TexturesLoaded {
			if loaded.Path == resolved {
				textures = append(textures, loaded)
				skip = true
				break
			}
		}
		if skip {
			continue
		}

		// One Acquire per distinct texture in this model, matched by one
		// Release in Model.Delete.
		textureId, isTransparent, err := tex.Acquire(resolved)
		if err != nil {
			return nil, fmt.Errorf("failed to load texture %q: %w", resolved, err)
		}

		texture := Texture{
			Id:              textureId,
			Path:            resolved,
			Type:            typeName,
			HasTransparency: isTransparent,
		}
		textures = append(textures, texture)
		m.TexturesLoaded = append(m.TexturesLoaded, texture)
	}

	return textures, nil
}
