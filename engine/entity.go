package engine

import (
	"3d-engine/object"

	"github.com/go-gl/mathgl/mgl32"
)

// MeshRenderer draws a shared model asset at its entity's world transform.
type MeshRenderer struct {
	Model *object.Model
}

// RigidBody makes an entity participate in the fixed-step physics pass. Static
// bodies are collided against but never moved.
type RigidBody struct {
	Velocity mgl32.Vec3
	Static   bool
}

// Entity is a node in the scene: a name, a placement, an optional parent, and
// the components attached to it. It replaces the transform fields that used to
// hang off object.Model, so the asset and its placement are no longer the same
// thing.
//
// The transform is behind accessors on purpose: every write has to invalidate
// the cached world matrix of this entity and everything below it, which direct
// field assignment would silently skip.
type Entity struct {
	ID   uint32
	Name string

	local    Transform
	parent   *Entity
	children []*Entity

	world      mgl32.Mat4
	worldDirty bool

	Renderer *MeshRenderer
	Body     *RigidBody
}

func NewEntity(name string) *Entity {
	return &Entity{
		Name:       name,
		local:      IdentityTransform(),
		worldDirty: true,
	}
}

func (e *Entity) Transform() Transform {
	return e.local
}

func (e *Entity) SetTransform(t Transform) {
	e.local = t
	e.invalidate()
}

func (e *Entity) Position() mgl32.Vec3 { return e.local.Position }
func (e *Entity) Rotation() mgl32.Vec4 { return e.local.Rotation }
func (e *Entity) Scale() mgl32.Vec3    { return e.local.Scale }

func (e *Entity) SetPosition(p mgl32.Vec3) {
	e.local.Position = p
	e.invalidate()
}

func (e *Entity) SetRotation(r mgl32.Vec4) {
	e.local.Rotation = r
	e.invalidate()
}

func (e *Entity) SetScale(s mgl32.Vec3) {
	e.local.Scale = s
	e.invalidate()
}

func (e *Entity) Translate(delta mgl32.Vec3) {
	e.SetPosition(e.local.Position.Add(delta))
}

// WorldMatrix returns the cached local-to-world matrix, rebuilding it only when
// this entity or an ancestor moved. Call it from the frame-loop goroutine: it
// writes the cache, so concurrent callers would race.
func (e *Entity) WorldMatrix() mgl32.Mat4 {
	if !e.worldDirty {
		return e.world
	}

	e.world = e.local.Matrix()
	if e.parent != nil {
		e.world = e.parent.WorldMatrix().Mul4(e.world)
	}
	e.worldDirty = false
	return e.world
}

// WorldAABB is the model's bounds placed in world space, or a point at the
// entity's position when it has nothing to render.
func (e *Entity) WorldAABB() object.AABB {
	if e.Renderer == nil || e.Renderer.Model == nil {
		return object.PointAABB(e.WorldMatrix().Col(3).Vec3())
	}
	return e.Renderer.Model.WorldAABB(e.WorldMatrix())
}

func (e *Entity) Parent() *Entity { return e.parent }

// Children returns the live child slice; treat it as read-only.
func (e *Entity) Children() []*Entity { return e.children }

// SetParent reparents the entity. The local transform is kept as-is, so the
// entity moves with its new parent rather than holding its world position.
func (e *Entity) SetParent(parent *Entity) {
	if e.parent == parent || parent == e {
		return
	}

	if e.parent != nil {
		siblings := e.parent.children
		for i, child := range siblings {
			if child == e {
				e.parent.children = append(siblings[:i], siblings[i+1:]...)
				break
			}
		}
	}

	e.parent = parent
	if parent != nil {
		parent.children = append(parent.children, e)
	}
	e.invalidate()
}

// invalidate marks this entity and its whole subtree as needing a rebuild.
func (e *Entity) invalidate() {
	e.worldDirty = true
	for _, child := range e.children {
		child.invalidate()
	}
}
