package engine

import (
	"3d-engine/object"

	"github.com/go-gl/mathgl/mgl32"
)

// DefaultBaseColor is the tint used for geometry with no diffuse texture. A
// neutral light grey, so an untextured mesh reads as a lit surface rather than
// as something wrong.
var DefaultBaseColor = mgl32.Vec3{0.8, 0.8, 0.8}

// MeshRenderer draws a shared model asset at its entity's world transform.
type MeshRenderer struct {
	Model *object.Model

	// BaseColor stands in for the diffuse texture on meshes that have none.
	//
	// It lives on the renderer rather than on the model because a model is a
	// shared asset: two entities can draw the same mesh in different colours,
	// and tinting the asset would tint every user of it.
	BaseColor mgl32.Vec3
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
	Name string

	// handle is assigned by the World on spawn and cleared on despawn.
	handle Handle

	local    Transform
	parent   *Entity
	children []*Entity

	world      mgl32.Mat4
	worldDirty bool

	// Renderer and Body are the built-in components. They stay typed fields
	// rather than entries in components so the render and physics loops can
	// reach them without a type assertion per entity per frame.
	Renderer *MeshRenderer
	Body     *RigidBody

	components []Component
	// unstarted holds components whose Start has not run yet.
	unstarted []Component
}

// AddComponent attaches a component. Its Start runs at the next update.
func (e *Entity) AddComponent(components ...Component) {
	for _, component := range components {
		if component == nil {
			continue
		}
		e.components = append(e.components, component)
		e.unstarted = append(e.unstarted, component)
	}
}

// Components returns the attached components; treat the slice as read-only.
func (e *Entity) Components() []Component {
	return e.components
}

// takeUnstarted returns the components awaiting Start and clears the queue.
func (e *Entity) takeUnstarted() []Component {
	if len(e.unstarted) == 0 {
		return nil
	}

	pending := e.unstarted
	e.unstarted = nil
	return pending
}

func NewEntity(name string) *Entity {
	return &Entity{
		Name:       name,
		local:      IdentityTransform(),
		worldDirty: true,
	}
}

// Handle is the entity's stable identity. It is the zero Handle until the
// entity is spawned into a World.
func (e *Entity) Handle() Handle {
	return e.handle
}

func (e *Entity) Transform() Transform {
	return e.local
}

func (e *Entity) SetTransform(t Transform) {
	e.local = t
	e.invalidate()
}

func (e *Entity) Position() mgl32.Vec3 { return e.local.Position }
func (e *Entity) Rotation() mgl32.Quat { return e.local.Rotation }
func (e *Entity) Scale() mgl32.Vec3    { return e.local.Scale }

func (e *Entity) SetPosition(p mgl32.Vec3) {
	e.local.Position = p
	e.invalidate()
}

func (e *Entity) SetRotation(r mgl32.Quat) {
	e.local.Rotation = r
	e.invalidate()
}

// RotationAxisAngle reports the rotation as XYZ axis plus degrees, the form the
// scene files, the gRPC surface and the inspector use.
func (e *Entity) RotationAxisAngle() mgl32.Vec4 {
	return AxisAngleFromQuat(e.local.Rotation)
}

// SetRotationAxisAngle sets the rotation from XYZ axis plus degrees.
func (e *Entity) SetRotationAxisAngle(axisAngle mgl32.Vec4) {
	e.SetRotation(QuatFromAxisAngle(axisAngle))
}

// Rotate composes a rotation onto the entity's current one.
//
// This is what the quaternion representation buys: with axis-angle there was no
// way to combine two rotations without going out to a matrix and being unable to
// get back, so anything that turned an entity had to overwrite whatever rotation
// was already there.
func (e *Entity) Rotate(delta mgl32.Quat) {
	e.SetRotation(delta.Mul(e.local.Rotation).Normalize())
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

// IsAncestorOf reports whether e is somewhere above other in the tree.
//
// It is what makes a cycle check possible. A cycle is not a cosmetic problem:
// WorldMatrix walks up the parent chain, so an entity parented into its own
// subtree recurses until the stack runs out.
func (e *Entity) IsAncestorOf(other *Entity) bool {
	for ancestor := other; ancestor != nil; ancestor = ancestor.parent {
		if ancestor == e {
			return true
		}
	}
	return false
}

// SetParent reparents the entity. The local transform is kept as-is, so the
// entity moves with its new parent rather than holding its world position.
//
// A parent that would form a cycle is refused, in keeping with the existing
// no-op for parenting to self. Callers that need to tell the difference between
// "done" and "refused" should go through App.SetParent, which reports it.
func (e *Entity) SetParent(parent *Entity) {
	if e.parent == parent || parent == e {
		return
	}
	if parent != nil && e.IsAncestorOf(parent) {
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
