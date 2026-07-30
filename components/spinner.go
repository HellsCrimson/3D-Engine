package components

import (
	"math"

	"3d-engine/engine"

	"github.com/go-gl/mathgl/mgl32"
)

// Spinner turns its entity about a fixed axis.
//
//	components:
//	  - type: Spinner
//	    props: { axis: [0, 1, 0], speed: 45 }
//
// The spinner owns its entity's rotation: it writes the whole rotation every
// frame from Angle and Axis, so an axis set in the transform block is replaced by
// this one. Angle carries over, because Start takes it as the starting point.
//
// Rotations are quaternions internally now, so composing this spin onto an
// authored rotation instead of replacing it is possible — it would need a second
// property to hold the base rotation, since the saved transform already includes
// the spin and could not be told apart from it on reload.
type Spinner struct {
	// Axis is the rotation axis in the entity's own space. It does not have to
	// be normalised; the matrix build normalises it.
	Axis mgl32.Vec3 `yaml:"axis"`

	// Speed is degrees per second. Negative spins the other way.
	Speed float32 `yaml:"speed"`

	// Angle is the current rotation in degrees. It is a property, not private
	// state, so that saving a scene mid-spin and loading it back resumes from
	// the same place instead of snapping to zero.
	Angle float32 `yaml:"angle"`
}

// NewSpinner returns a spinner making one revolution every four seconds about Y,
// which is a rate you can see without it being distracting.
func NewSpinner() *Spinner {
	return &Spinner{
		Axis:  mgl32.Vec3{0, 1, 0},
		Speed: 90,
	}
}

// Start seeds the angle from the transform, so `rotation: [0, 1, 0, 45]` with no
// `angle` property starts a quarter-turn in rather than jumping to zero on the
// first frame.
//
// It only does so when the property was left at its default, otherwise an
// explicit `angle: 0` could not mean zero.
func (s *Spinner) Start(ctx *engine.Context) {
	if s.Angle == 0 {
		s.Angle = ctx.Entity.RotationAxisAngle().W()
	}
}

// Update advances the spin. Writing through the accessor rather than the field is
// what invalidates the cached world matrix of this entity and its children.
func (s *Spinner) Update(ctx *engine.Context) {
	axis := s.Axis
	if axis.Len() == 0 {
		// A scene that says `axis: [0, 0, 0]` would otherwise build a rotation
		// full of NaN and the entity would vanish. The engine guards this too;
		// keeping it here means Axis stays meaningful to read back.
		axis = mgl32.Vec3{0, 1, 0}
	}

	s.Angle = wrapDegrees(s.Angle + s.Speed*ctx.DeltaTime)
	ctx.Entity.SetRotationAxisAngle(mgl32.Vec4{axis.X(), axis.Y(), axis.Z(), s.Angle})
}

// wrapDegrees folds an angle into [0, 360). Without it a spinner left running
// accumulates float error, and the angle written into a saved scene grows without
// bound.
func wrapDegrees(degrees float32) float32 {
	wrapped := float32(math.Mod(float64(degrees), 360))
	if wrapped < 0 {
		wrapped += 360
	}
	return wrapped
}
