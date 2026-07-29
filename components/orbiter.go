package components

import (
	"math"

	"3d-engine/engine"

	"github.com/go-gl/mathgl/mgl32"
)

// Orbiter moves its entity in a circle. Attached to an entity carrying a
// PointLight it is the clearest demonstration the engine has that components run
// at all: the light sweeps across the geometry and the shading follows it.
//
//	components:
//	  - type: PointLight
//	    props: { diffuse: [1.0, 0.6, 0.2] }
//	  - type: Orbiter
//	    props: { center: [0, 4, 0], radius: 6, speed: 30 }
//
// The orbiter writes the entity's local position, so Center is in the parent's
// space — for an entity with no parent that is world space.
type Orbiter struct {
	// Center is the point orbited, in the same space as the entity's own
	// position.
	Center mgl32.Vec3 `yaml:"center"`

	// Axis is the normal of the orbital plane: the default Y sweeps the entity
	// around horizontally.
	Axis mgl32.Vec3 `yaml:"axis"`

	Radius float32 `yaml:"radius"`

	// Speed is degrees per second. Negative orbits the other way.
	Speed float32 `yaml:"speed"`

	// Phase is the current position around the circle, in degrees. Set it in a
	// scene file to space several orbiters out around the same centre.
	//
	// It is a property rather than private state so a saved scene resumes the
	// orbit where it left off. Without that, reloading would put the entity back
	// at the saved position for one frame and then snap it to phase zero.
	Phase float32 `yaml:"phase"`
}

// NewOrbiter returns a slow, wide horizontal orbit.
func NewOrbiter() *Orbiter {
	return &Orbiter{
		Axis:   mgl32.Vec3{0, 1, 0},
		Radius: 5,
		Speed:  45,
	}
}

// Update advances the orbit and places the entity.
func (o *Orbiter) Update(ctx *engine.Context) {
	o.Phase = wrapDegrees(o.Phase + o.Speed*ctx.DeltaTime)

	first, second := orbitBasis(o.Axis)
	radians := float64(mgl32.DegToRad(o.Phase))

	offset := first.Mul(float32(math.Cos(radians))).
		Add(second.Mul(float32(math.Sin(radians))))

	ctx.Entity.SetPosition(o.Center.Add(offset.Mul(o.Radius)))
}

// orbitBasis returns two perpendicular unit vectors spanning the plane the axis
// is normal to. Phase 0 puts the entity along the first, and a growing phase
// sweeps it towards the second.
//
// The reference vector is swapped when the axis is nearly parallel to Y, because
// the cross product of two parallel vectors is zero and normalising that gives
// NaN — which would place the entity nowhere and, since the render loop feeds the
// position straight into a matrix, quietly break the frame.
func orbitBasis(axis mgl32.Vec3) (first, second mgl32.Vec3) {
	normal := mgl32.Vec3{0, 1, 0}
	if axis.Len() > 0 {
		normal = axis.Normalize()
	}

	reference := mgl32.Vec3{0, 1, 0}
	if math.Abs(float64(normal.Y())) > 0.99 {
		reference = mgl32.Vec3{1, 0, 0}
	}

	first = reference.Cross(normal).Normalize()
	second = normal.Cross(first)
	return first, second
}
