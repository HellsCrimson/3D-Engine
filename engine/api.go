package engine

import (
	"fmt"

	"3d-engine/object"
	"3d-engine/utils"
)

// This file is the engine's object API: the one place an entity is built,
// destroyed or queried. Scene files, the gRPC server, the editor overlay and
// game code all come through here, so each operation exists once and no
// front-end can do something another cannot.

// ObjectSpec describes an entity to create. A row in a scene file, an
// ADD_OBJECT request and a Go SpawnObject call all turn into one of these.
type ObjectSpec struct {
	Name string

	// Model is the asset path. Empty means no renderer — a light or a pure
	// logic node.
	Model string

	Transform Transform

	// Body is optional. Without one the entity is not integrated by the physics
	// step, which is what keeps a light from falling.
	Body *RigidBody

	Components []Component
}

// ObjectInfo is a read-only snapshot, taken under the world lock so callers
// never hold a live entity pointer.
type ObjectInfo struct {
	Handle    Handle
	Name      string
	Model     string
	Transform Transform
}

// BuildObject constructs an entity and acquires its assets without adding it to
// the world. Scene loading uses this to assemble a whole scene before swapping
// it in.
//
// It imports assets, so it must run on the frame-loop goroutine.
func (a *App) BuildObject(spec ObjectSpec) (*Entity, error) {
	entity := NewEntity(spec.Name)
	entity.SetTransform(spec.Transform)

	if spec.Model != "" {
		model, err := a.Assets.Acquire(spec.Model)
		if err != nil {
			return nil, fmt.Errorf("could not load model %q: %w", spec.Model, err)
		}
		entity.Renderer = &MeshRenderer{Model: model}
	}

	if spec.Body != nil {
		body := *spec.Body
		entity.Body = &body
	}

	entity.AddComponent(spec.Components...)
	return entity, nil
}

// SpawnObject builds an entity and adds it to the world. Frame-loop goroutine
// only; from anywhere else use Spawn.
func (a *App) SpawnObject(spec ObjectSpec) (*Entity, error) {
	entity, err := a.BuildObject(spec)
	if err != nil {
		return nil, err
	}

	a.World.Spawn(entity)
	return entity, nil
}

// Spawn queues SpawnObject onto the frame loop and waits for the handle. Safe
// from any goroutine except the frame loop itself, which would deadlock.
func (a *App) Spawn(spec ObjectSpec) (Handle, error) {
	var handle Handle

	err := a.Do(func(app *App) error {
		entity, err := app.SpawnObject(spec)
		if err != nil {
			return err
		}
		handle = entity.Handle()
		return nil
	})

	return handle, err
}

// DespawnObject removes an entity and releases the model it was holding.
// Frame-loop goroutine only; from anywhere else use Despawn.
//
// Releasing here is what makes despawn symmetric with spawn: World.Despawn
// alone would drop the entity and leak its GPU memory.
func (a *App) DespawnObject(handle Handle) error {
	entity := a.World.Get(handle)
	if entity == nil {
		return fmt.Errorf("object %s not found", handle)
	}

	// Grab the model before despawning, since the entity is gone afterwards.
	var model *object.Model
	if entity.Renderer != nil {
		model = entity.Renderer.Model
	}

	if !a.World.Despawn(handle) {
		return fmt.Errorf("object %s not found", handle)
	}

	if model != nil {
		if err := a.Assets.Release(model); err != nil {
			utils.Logger().Printf("Releasing model %s: %v", model.Path, err)
		}
	}
	return nil
}

// Despawn queues DespawnObject onto the frame loop and waits.
func (a *App) Despawn(handle Handle) error {
	return a.Do(func(app *App) error {
		return app.DespawnObject(handle)
	})
}

// UpdateTransform applies fn to the entity's transform under the write lock.
// Safe from any goroutine: it only touches fields, never GL.
func (a *App) UpdateTransform(handle Handle, fn func(t *Transform)) error {
	ok := a.World.Mutate(handle, func(entity *Entity) {
		transform := entity.Transform()
		fn(&transform)
		entity.SetTransform(transform)
	})

	if !ok {
		return fmt.Errorf("object %s not found", handle)
	}
	return nil
}

// ObjectInfo snapshots one entity.
func (a *App) ObjectInfo(handle Handle) (ObjectInfo, bool) {
	var info ObjectInfo
	found := false

	a.World.Mutate(handle, func(entity *Entity) {
		info = describe(entity)
		found = true
	})

	return info, found
}

// ListObjects snapshots every entity in the world.
func (a *App) ListObjects() []ObjectInfo {
	var objects []ObjectInfo

	a.World.Read(func(entities []*Entity) {
		objects = make([]ObjectInfo, 0, len(entities))
		for _, entity := range entities {
			objects = append(objects, describe(entity))
		}
	})

	return objects
}

func describe(entity *Entity) ObjectInfo {
	info := ObjectInfo{
		Handle:    entity.Handle(),
		Name:      entity.Name,
		Transform: entity.Transform(),
	}
	if entity.Renderer != nil && entity.Renderer.Model != nil {
		info.Model = entity.Renderer.Model.Path
	}
	return info
}
