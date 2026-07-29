package engine

import (
	"fmt"
	"reflect"

	"3d-engine/scene"
	"3d-engine/utils"

	"gopkg.in/yaml.v3"
)

// This file is the inverse of scene_manager.go's buildEntity: it turns live
// world state back into the scene format. The two have to stay in step, so a
// scene that was saved and reloaded produces the same world it was taken from.

// SceneSnapshot describes the live world in the scene file format.
//
// It reads the world under the read lock, but also reads the camera and the
// skybox, which are only written on the frame loop. Call it from the frame loop
// or through App.Defer.
//
// The camera it records is the live one, not the spawn point the scene was
// loaded with: saving from the editor is meant to capture the world as it looks
// right now, and where you are standing is part of that.
func (a *App) SceneSnapshot() (*scene.Scene, error) {
	snapshot := &scene.Scene{
		Version: scene.CurrentVersion,
	}

	if a.skybox != nil {
		snapshot.Skybox = a.skybox.Path
	}

	camera := scene.DefaultCamera()
	if a.Camera != nil {
		camera = scene.CameraSpec{
			Position: [3]float32(a.Camera.CameraPos),
			Yaw:      a.Camera.Yaw,
			Pitch:    a.Camera.Pitch,
		}
	}
	snapshot.Camera = &camera

	var err error
	a.World.Read(func(entities []*Entity) {
		snapshot.Objects = make([]scene.Object, 0, len(entities))

		for _, entity := range entities {
			var row scene.Object

			row, err = a.describeForSave(entity)
			if err != nil {
				return
			}
			snapshot.Objects = append(snapshot.Objects, row)
		}
	})
	if err != nil {
		return nil, err
	}

	return snapshot, nil
}

// SaveScene writes the live world to a scene file. Frame-loop goroutine only,
// for the same reason as SceneSnapshot.
func (a *App) SaveScene(path string) error {
	if path == "" {
		return fmt.Errorf("no scene path to save to")
	}

	snapshot, err := a.SceneSnapshot()
	if err != nil {
		return err
	}

	if err := scene.Save(path, snapshot); err != nil {
		return err
	}

	utils.Logger().Printf("Saved scene to %s", path)
	return nil
}

// describeForSave turns one entity into its scene-file row.
func (a *App) describeForSave(entity *Entity) (scene.Object, error) {
	transform := entity.Transform()

	row := scene.Object{
		Name: entity.Name,
		Transform: &scene.TransformSpec{
			Position: [3]float32(transform.Position),
			Rotation: [4]float32(transform.Rotation),
			Scale:    [3]float32(transform.Scale),
		},
	}

	// The path the model was imported with, not the asset cache's absolute key,
	// so a scene written with relative paths stays relative.
	if entity.Renderer != nil && entity.Renderer.Model != nil {
		row.Model = entity.Renderer.Model.Path
	}

	if entity.Body != nil {
		row.Body = &scene.BodySpec{Static: entity.Body.Static}
	}

	for _, component := range entity.components {
		spec, err := a.describeComponent(entity, component)
		if err != nil {
			return scene.Object{}, err
		}
		row.Components = append(row.Components, spec)
	}

	return row, nil
}

// describeComponent writes a live component back out under the name a scene file
// would use to ask for it.
//
// An unregistered component is an error rather than a silent omission: dropping
// it would produce a file that loads cleanly and quietly lost behaviour, which
// is far harder to notice than a refused save.
func (a *App) describeComponent(entity *Entity, component Component) (scene.ComponentSpec, error) {
	name, ok := a.Components.NameOf(component)
	if !ok {
		return scene.ComponentSpec{}, fmt.Errorf(
			"object %q has a component of type %s that is not registered, so it cannot be named in a scene file",
			entity.Name, reflect.TypeOf(component))
	}

	spec := scene.ComponentSpec{Type: name}

	var props yaml.Node
	if err := props.Encode(component); err != nil {
		return scene.ComponentSpec{}, fmt.Errorf(
			"object %q: encoding component %q: %w", entity.Name, name, err)
	}

	// A component with no exported fields encodes to an empty mapping. Leaving
	// the node zero keeps it a bare `type:` line instead of `props: {}`.
	if props.Kind != yaml.MappingNode || len(props.Content) > 0 {
		spec.Props = props
	}

	return spec, nil
}
