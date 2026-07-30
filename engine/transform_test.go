package engine

import (
	"math"
	"testing"

	"github.com/go-gl/mathgl/mgl32"
)

func nearly(got, want, tolerance float32) bool {
	return math.Abs(float64(got-want)) <= float64(tolerance)
}

func vec3Nearly(got, want mgl32.Vec3, tolerance float32) bool {
	return nearly(got.X(), want.X(), tolerance) &&
		nearly(got.Y(), want.Y(), tolerance) &&
		nearly(got.Z(), want.Z(), tolerance)
}

// representativeRotations covers the cases the conversion has to survive: the
// identity every unrotated object sits at, the cardinal axes scene files use, a
// non-normalised axis, a negative angle, an angle past a half turn, and an
// arbitrary tilted axis.
var representativeRotations = []mgl32.Vec4{
	{0, 1, 0, 0},
	{0, 1, 0, 45},
	{0, 1, 0, 90},
	{1, 0, 0, 45},
	{0, 0, 1, 180},
	{0, 2, 0, 30},
	{0, 1, 0, -45},
	{0, 1, 0, 270},
	{1, 1, 1, 120},
	{0.3, -0.7, 0.2, 33.5},
}

// TestAxisAngleConversionIsStable is the property the scene format rests on.
//
// The conversion is not required to give back the axis-angle pair it was handed —
// several pairs name the same rotation — but repeating it must not keep changing
// the answer, because a scene is loaded and saved over and over.
//
// The tolerance is deliberate. A trip through the trigonometry rounds at every
// step, so the last digit or two of a float32 can move; what must not happen is
// an axis flipping sign or an angle drifting somewhere new, which is what this
// catches.
func TestAxisAngleConversionIsStable(t *testing.T) {
	for _, original := range representativeRotations {
		once := AxisAngleFromQuat(QuatFromAxisAngle(original))
		twice := AxisAngleFromQuat(QuatFromAxisAngle(once))

		for i := 0; i < 4; i++ {
			if !nearly(once[i], twice[i], 1e-3) {
				t.Errorf("%v is not stable: first %v, then %v", original, once, twice)
				break
			}
		}
	}
}

// TestHalfTurnSignIsStable pins the one instability that was a real defect rather
// than rounding: at exactly 180 degrees the scalar part of the quaternion is zero,
// and float32 puts it a hair below. Taking the canonical sign from it made a half
// turn alternate between an axis and its opposite every time the scene was saved.
func TestHalfTurnSignIsStable(t *testing.T) {
	halfTurns := []mgl32.Vec4{
		{0, 0, 1, 180},
		{0, 1, 0, 180},
		{1, 0, 0, 180},
		{0, 0, -1, 180},
		{1, 1, 0, 180},
	}

	for _, original := range halfTurns {
		once := AxisAngleFromQuat(QuatFromAxisAngle(original))
		twice := AxisAngleFromQuat(QuatFromAxisAngle(once))

		if once != twice {
			t.Errorf("half turn %v flips between saves: %v then %v", original, once, twice)
		}
	}
}

// TestAxisAngleKeepsTheRotation checks the part that actually matters: whatever
// pair comes back has to turn a vector to the same place as the pair that went
// in.
func TestAxisAngleKeepsTheRotation(t *testing.T) {
	probes := []mgl32.Vec3{{1, 0, 0}, {0, 1, 0}, {0, 0, 1}, {1, 2, 3}}

	for _, original := range representativeRotations {
		from := QuatFromAxisAngle(original)
		to := QuatFromAxisAngle(AxisAngleFromQuat(from))

		for _, probe := range probes {
			want := from.Rotate(probe)
			got := to.Rotate(probe)
			if !vec3Nearly(got, want, 1e-4) {
				t.Errorf("rotation %v applied to %v: got %v, want %v", original, probe, got, want)
			}
		}
	}
}

// TestQuaternionMatrixMatchesAxisAngle is the regression guard for the migration
// itself: the matrix a transform now builds from a quaternion has to be the one
// it used to build with HomogRotate3D, or every object in every scene would move.
func TestQuaternionMatrixMatchesAxisAngle(t *testing.T) {
	for _, rotation := range representativeRotations {
		axis := rotation.Vec3()
		if axis.Len() == 0 {
			continue
		}

		// What Transform.Matrix used to do.
		want := mgl32.HomogRotate3D(mgl32.DegToRad(rotation.W()), axis.Normalize())
		got := QuatFromAxisAngle(rotation).Mat4()

		for i := 0; i < 16; i++ {
			if !nearly(got[i], want[i], 1e-5) {
				t.Fatalf("rotation %v: matrix element %d is %v, want %v\ngot  %v\nwant %v",
					rotation, i, got[i], want[i], got, want)
			}
		}
	}
}

// TestIdentityRotationCanonicalisesToY keeps unrotated objects reading the way
// every hand-written scene file already writes them.
func TestIdentityRotationCanonicalisesToY(t *testing.T) {
	got := AxisAngleFromQuat(mgl32.QuatIdent())
	if got != (mgl32.Vec4{0, 1, 0, 0}) {
		t.Errorf("identity rotation: got %v, want {0 1 0 0}", got)
	}

	// The zero quaternion is not a valid rotation, but a bare Transform{} has
	// one, so it must not produce NaN either.
	got = AxisAngleFromQuat(mgl32.Quat{})
	if got != (mgl32.Vec4{0, 1, 0, 0}) {
		t.Errorf("zero quaternion: got %v, want {0 1 0 0}", got)
	}
}

// TestNegativeAngleCanonicalises documents what the conversion deliberately does
// not preserve: the same rotation always comes back described the same way.
func TestNegativeAngleCanonicalises(t *testing.T) {
	got := AxisAngleFromQuat(QuatFromAxisAngle(mgl32.Vec4{0, 1, 0, -45}))

	if !nearly(got.W(), 45, 1e-4) {
		t.Errorf("angle: got %v, want 45 with the sign moved onto the axis", got.W())
	}
	if !vec3Nearly(got.Vec3(), mgl32.Vec3{0, -1, 0}, 1e-4) {
		t.Errorf("axis: got %v, want {0 -1 0}", got.Vec3())
	}
}

// TestAngleSpansTheWholeTurn keeps a rotation past a half turn reading as one.
//
// Folding the angle into [0, 180] and moving the sign onto the axis names the
// same rotation and would be just as correct, but it makes a spinning entity
// report an angle that climbs to 180 and then comes back down with its axis
// flipped. That is what a person watching the inspector sees, and what a gRPC
// client reads.
func TestAngleSpansTheWholeTurn(t *testing.T) {
	for _, angle := range []float32{30, 90, 179, 181, 200, 270, 350} {
		got := AxisAngleFromQuat(QuatFromAxisAngle(mgl32.Vec4{0, 1, 0, angle}))

		if !nearly(got.W(), angle, 1e-2) {
			t.Errorf("%v degrees about Y came back as %v degrees", angle, got.W())
		}
		if !vec3Nearly(got.Vec3(), mgl32.Vec3{0, 1, 0}, 1e-4) {
			t.Errorf("%v degrees about Y came back about %v", angle, got.Vec3())
		}
	}
}

// TestFullTurnReadsAsNoRotation covers the wrap point, where the quaternion is
// the negated identity rather than the identity.
func TestFullTurnReadsAsNoRotation(t *testing.T) {
	got := AxisAngleFromQuat(QuatFromAxisAngle(mgl32.Vec4{0, 1, 0, 360}))

	if !nearly(got.W(), 0, 1e-2) {
		t.Errorf("a full turn should read as no rotation, got %v degrees", got.W())
	}
	if got.Vec3() != (mgl32.Vec3{0, 1, 0}) {
		t.Errorf("a full turn should report the default axis, got %v", got.Vec3())
	}
}

// TestZeroAxisFallsBackToY guards the NaN path. mgl32 normalises a zero vector to
// NaN, and a NaN rotation makes the entity vanish from the render instead of
// reporting anything.
func TestZeroAxisFallsBackToY(t *testing.T) {
	q := QuatFromAxisAngle(mgl32.Vec4{0, 0, 0, 90})

	if math.IsNaN(float64(q.W)) || math.IsNaN(float64(q.V[0])) ||
		math.IsNaN(float64(q.V[1])) || math.IsNaN(float64(q.V[2])) {
		t.Fatalf("zero axis produced NaN: %v", q)
	}

	got := AxisAngleFromQuat(q)
	if !vec3Nearly(got.Vec3(), mgl32.Vec3{0, 1, 0}, 1e-4) || !nearly(got.W(), 90, 1e-4) {
		t.Errorf("zero axis should become 90 degrees about Y, got %v", got)
	}
}

// TestNonNormalisedAxisIsAccepted covers a scene file written by hand with a
// convenient but non-unit axis.
func TestNonNormalisedAxisIsAccepted(t *testing.T) {
	loose := QuatFromAxisAngle(mgl32.Vec4{0, 5, 0, 60})
	tight := QuatFromAxisAngle(mgl32.Vec4{0, 1, 0, 60})

	probe := mgl32.Vec3{1, 0, 0}
	if !vec3Nearly(loose.Rotate(probe), tight.Rotate(probe), 1e-5) {
		t.Errorf("axis length should not change the rotation: %v vs %v",
			loose.Rotate(probe), tight.Rotate(probe))
	}
}

// TestRotationsCompose is what the quaternion representation was for. Two
// rotations could not be combined in axis-angle form at all, which is why
// anything that turned an entity had to overwrite whatever was already there.
func TestRotationsCompose(t *testing.T) {
	entity := NewEntity("subject")
	entity.SetRotationAxisAngle(mgl32.Vec4{0, 1, 0, 45})

	step := mgl32.QuatRotate(mgl32.DegToRad(45), mgl32.Vec3{0, 1, 0})
	entity.Rotate(step)

	got := entity.RotationAxisAngle()
	if !nearly(got.W(), 90, 1e-3) {
		t.Errorf("45 degrees composed onto 45 should be 90, got %v", got.W())
	}
	if !vec3Nearly(got.Vec3(), mgl32.Vec3{0, 1, 0}, 1e-4) {
		t.Errorf("axis should stay Y, got %v", got.Vec3())
	}
}

// TestComposedRotationsAboutDifferentAxes is the case that has no axis-angle
// answer at all: the result is about an axis neither input mentions.
func TestComposedRotationsAboutDifferentAxes(t *testing.T) {
	entity := NewEntity("subject")
	entity.SetRotationAxisAngle(mgl32.Vec4{0, 1, 0, 90})
	entity.Rotate(mgl32.QuatRotate(mgl32.DegToRad(90), mgl32.Vec3{1, 0, 0}))

	// Turning about Y takes +X to -Z, and then turning about X takes -Z to +Y.
	// Neither rotation alone sends +X anywhere near +Y, so this only passes if
	// both were applied.
	got := entity.Rotation().Rotate(mgl32.Vec3{1, 0, 0})
	if !vec3Nearly(got, mgl32.Vec3{0, 1, 0}, 1e-4) {
		t.Errorf("composed rotation applied to +X: got %v, want {0 1 0}", got)
	}

	// And the result stays a unit quaternion, so repeated composition cannot
	// drift into a scaling matrix.
	if !nearly(entity.Rotation().Len(), 1, 1e-5) {
		t.Errorf("composed rotation is not unit length: %v", entity.Rotation().Len())
	}
}

// TestRepeatedCompositionStaysUnit walks a full turn in small steps, which is
// what a spinning component does over a few seconds of frames.
func TestRepeatedCompositionStaysUnit(t *testing.T) {
	entity := NewEntity("subject")
	entity.SetRotation(mgl32.QuatIdent())

	step := mgl32.QuatRotate(mgl32.DegToRad(1), mgl32.Vec3{0, 1, 0})
	for i := 0; i < 360; i++ {
		entity.Rotate(step)
	}

	if !nearly(entity.Rotation().Len(), 1, 1e-4) {
		t.Errorf("after 360 compositions the rotation is not unit: %v", entity.Rotation().Len())
	}

	// A full turn is back to the identity, which reads as an angle of zero.
	got := entity.RotationAxisAngle()
	if !nearly(got.W(), 0, 0.1) && !nearly(got.W(), 360, 0.1) {
		t.Errorf("a full turn should come back to no rotation, got %v", got)
	}
}

// TestSweepIsStable runs the fixed-point check across a spread of angles and
// axes rather than the handful above, since the failure it guards was a few ulps
// wide and only showed up for particular values.
func TestSweepIsStable(t *testing.T) {
	axes := []mgl32.Vec3{
		{1, 0, 0}, {0, 1, 0}, {0, 0, 1},
		{1, 1, 0}, {1, 0, 1}, {0, 1, 1}, {1, 1, 1},
		{-1, 2, -3}, {0.001, 1, 0},
	}

	for _, axis := range axes {
		for angle := -350; angle <= 350; angle += 7 {
			original := mgl32.Vec4{axis.X(), axis.Y(), axis.Z(), float32(angle)}

			once := AxisAngleFromQuat(QuatFromAxisAngle(original))
			twice := AxisAngleFromQuat(QuatFromAxisAngle(once))

			for i := 0; i < 4; i++ {
				if !nearly(once[i], twice[i], 1e-3) {
					t.Fatalf("axis %v angle %d is not stable: %v then %v",
						axis, angle, once, twice)
				}
			}
		}
	}
}
