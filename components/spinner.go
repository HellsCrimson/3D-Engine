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
// The spinner owns its entity's rotation outright: it writes the whole
// axis-angle vector every frame, so an axis set in the transform block is
// replaced by this one. That is a consequence of rotations being stored as
// axis-angle — two of them cannot be composed without going through
// quaternions. Angle is the one part of the transform's rotation that carries
// over, because the spinner takes it as its starting point.
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
		s.Angle = ctx.Entity.Rotation().W()
	}
}

// Update advances the spin. Writing through SetRotation rather than the field is
// what invalidates the cached world matrix of this entity and its children.
func (s *Spinner) Update(ctx *engine.Context) {
	axis := s.Axis
	if axis.Len() == 0 {
		// A scene that says `axis: [0, 0, 0]` would otherwise build a matrix
		// full of NaN and the entity would vanish.
		axis = mgl32.Vec3{0, 1, 0}
	}

	s.Angle = wrapDegrees(s.Angle + s.Speed*ctx.DeltaTime)
	ctx.Entity.SetRotation(mgl32.Vec4{axis.X(), axis.Y(), axis.Z(), s.Angle})
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
