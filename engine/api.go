package engine

import (
	"fmt"

	"3d-engine/object"
	"3d-engine/utils"

	"github.com/go-gl/mathgl/mgl32"
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

	// BaseColor tints the parts of the model that carry no diffuse texture. Nil
	// keeps DefaultBaseColor, which is what lets an omitted material block mean
	// "the usual grey" rather than black.
	BaseColor *mgl32.Vec3

	Transform Transform

	// Body is optional. Without one the entity is not integrated by the physics
	// step, which is what keeps a light from falling.
	Body *RigidBody

	Components []Component

	// Children are built with this entity as their parent. A scene file's nested
	// `children:` block and a spawn of a whole prefab-like tree are the same
	// thing here.
	Children []ObjectSpec

	// Parent, when set, attaches the new entity under an entity that is already
	// in the world. It is resolved before anything is spawned, so a stale handle
	// fails without leaving a half-attached entity behind.
	Parent Handle
}

// ObjectInfo is a read-only snapshot, taken under the world lock so callers
// never hold a live entity pointer.
type ObjectInfo struct {
	Handle    Handle
	Name      string
	Model     string
	Transform Transform

	// BaseColor is the renderer's tint for untextured geometry. It is the zero
	// vector for an entity with no renderer.
	BaseColor mgl32.Vec3

	// Parent is NoHandle for an entity at the root of the scene.
	Parent Handle

	// Children are handles rather than nested ObjectInfos so a caller can walk
	// the tree at its own pace, and so one deep subtree does not make every
	// snapshot expensive.
	Children []Handle
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

		renderer := &MeshRenderer{Model: model, BaseColor: DefaultBaseColor}
		if spec.BaseColor != nil {
			renderer.BaseColor = *spec.BaseColor
		}
		entity.Renderer = renderer
	}

	if spec.Body != nil {
		body := *spec.Body
		entity.Body = &body
	}

	entity.AddComponent(spec.Components...)
	return entity, nil
}

// BuildTree builds a spec and everything under it, returning one flat list with
// the parent links already wired. The root is first.
//
// The list is flat because the World holds every entity in one dense slice
// regardless of shape — the tree lives in the entities' parent pointers, not in
// how they are stored. Scene loading needs exactly this: build the whole scene,
// then hand it to World.Replace in a single swap.
//
// It imports assets, so it must run on the frame-loop goroutine. A failure
// anywhere releases what this call already acquired and returns nothing, so a
// partly built tree never leaks models.
func (a *App) BuildTree(spec ObjectSpec) ([]*Entity, error) {
	root, err := a.BuildObject(spec)
	if err != nil {
		return nil, err
	}

	built := []*Entity{root}

	for i := range spec.Children {
		subtree, err := a.BuildTree(spec.Children[i])
		if err != nil {
			a.releaseModels(built)
			return nil, fmt.Errorf("child of %q: %w", spec.Name, err)
		}

		subtree[0].SetParent(root)
		built = append(built, subtree...)
	}

	return built, nil
}

// SpawnObject builds an entity, and everything under it, and adds it all to the
// world. It returns the root. Frame-loop goroutine only; from anywhere else use
// Spawn.
func (a *App) SpawnObject(spec ObjectSpec) (*Entity, error) {
	entities, err := a.BuildTree(spec)
	if err != nil {
		return nil, err
	}
	root := entities[0]

	// Resolved before anything is spawned: a stale parent handle should fail
	// cleanly rather than leave the new entity in the world unattached.
	if !spec.Parent.IsZero() {
		attached := a.World.Mutate(spec.Parent, func(parent *Entity) {
			root.SetParent(parent)
		})
		if !attached {
			a.releaseModels(entities)
			return nil, fmt.Errorf("parent %s not found", spec.Parent)
		}
	}

	for _, entity := range entities {
		a.World.Spawn(entity)
	}

	return root, nil
}

// releaseModels returns each entity's model to the asset cache. It is the
// teardown half of BuildObject, used for the outgoing scene after a swap, for a
// build that failed part-way, and at shutdown.
func (a *App) releaseModels(entities []*Entity) {
	for _, entity := range entities {
		if entity.Renderer == nil || entity.Renderer.Model == nil {
			continue
		}
		if err := a.Assets.Release(entity.Renderer.Model); err != nil {
			utils.Logger().Printf("Releasing model: %v", err)
		}
	}
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

// DespawnTree removes an entity together with everything below it.
//
// DespawnObject on its own orphans the children up to the scene root, which is
// the right primitive but almost never what deleting means: an editor's Delete,
// or a prefab going away, should take the subtree with it.
//
// Frame-loop goroutine only; from anywhere else use DespawnSubtree.
func (a *App) DespawnTree(handle Handle) error {
	var order []Handle

	found := a.World.Mutate(handle, func(root *Entity) {
		order = subtreeHandles(root)
	})
	if !found {
		return fmt.Errorf("object %s not found", handle)
	}

	// Deepest first, so every entity is childless by the time it is removed and
	// World.Despawn's orphaning step has nothing to do.
	for _, h := range order {
		if err := a.DespawnObject(h); err != nil {
			return err
		}
	}
	return nil
}

// DespawnSubtree queues DespawnTree onto the frame loop and waits.
func (a *App) DespawnSubtree(handle Handle) error {
	return a.Do(func(app *App) error {
		return app.DespawnTree(handle)
	})
}

// subtreeHandles lists the subtree children-first. Callers hold the world lock.
func subtreeHandles(root *Entity) []Handle {
	var order []Handle

	for _, child := range root.Children() {
		order = append(order, subtreeHandles(child)...)
	}
	return append(order, root.Handle())
}

// SetParent reparents an entity, or detaches it to the scene root when parent is
// NoHandle. Safe from any goroutine: it only relinks pointers, never touches GL.
func (a *App) SetParent(child, parent Handle) error {
	return a.World.Reparent(child, parent)
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

// SetBaseColor changes the tint used where an entity's model has no diffuse
// texture. Safe from any goroutine: it writes a field the render loop reads, and
// touches no GL.
func (a *App) SetBaseColor(handle Handle, color mgl32.Vec3) error {
	var hasRenderer bool

	found := a.World.Mutate(handle, func(entity *Entity) {
		if entity.Renderer == nil {
			return
		}
		entity.Renderer.BaseColor = color
		hasRenderer = true
	})

	if !found {
		return fmt.Errorf("object %s not found", handle)
	}
	if !hasRenderer {
		return fmt.Errorf("object %s has no model to colour", handle)
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
	if entity.Renderer != nil {
		info.BaseColor = entity.Renderer.BaseColor
		if entity.Renderer.Model != nil {
			info.Model = entity.Renderer.Model.Path
		}
	}
	if parent := entity.Parent(); parent != nil {
		info.Parent = parent.Handle()
	}
	for _, child := range entity.Children() {
		info.Children = append(info.Children, child.Handle())
	}
	return info
}
