package engine

import (
	"math"

	"github.com/go-gl/mathgl/mgl32"
)

// Transform is a local placement.
//
// Rotation is a quaternion. Axis-angle survives at the edges — scene files, the
// gRPC surface and the editor's inspector all still speak XYZ axis plus degrees —
// but it is converted on the way in and out rather than stored.
//
// The reason is composition. Two axis-angle rotations cannot be combined without
// leaving the representation, which is why the parent/child chain has to go
// through matrices and why a world matrix cannot be decomposed back into a
// transform. Quaternions compose by multiplication, so Rotate is a one-liner and
// nothing has to guess an axis back out of a matrix.
type Transform struct {
	Position mgl32.Vec3
	Rotation mgl32.Quat
	Scale    mgl32.Vec3
}

// IdentityTransform is the sane default for an entity created in code: unit
// scale rather than the zero scale a bare Transform{} would give, and a real
// identity rotation rather than the zero quaternion.
func IdentityTransform() Transform {
	return Transform{
		Rotation: mgl32.QuatIdent(),
		Scale:    mgl32.Vec3{1, 1, 1},
	}
}

// Matrix builds the translate -> rotate -> scale matrix. This used to be
// open-coded in three places (the render loop, the player loop and
// Model.worldTransform); it lives here now so the order can only be changed
// once.
func (t Transform) Matrix() mgl32.Mat4 {
	mat := mgl32.Translate3D(t.Position.X(), t.Position.Y(), t.Position.Z())
	mat = mat.Mul4(t.Rotation.Mat4())
	return mat.Mul4(mgl32.Scale3D(t.Scale.X(), t.Scale.Y(), t.Scale.Z()))
}

// defaultAxis is the axis reported for a rotation of no angle, where the axis is
// mathematically arbitrary. Y matches what every hand-written scene file already
// says, so an unrotated object keeps reading as `rotation: [0, 1, 0, 0]` after a
// save rather than flipping to some other axis.
var defaultAxis = mgl32.Vec3{0, 1, 0}

// QuatFromAxisAngle converts the edge representation — XYZ axis, W angle in
// degrees — into a rotation.
//
// A zero axis falls back to Y instead of normalising to NaN. mgl32 would happily
// produce a quaternion full of NaN, and a NaN transform makes the entity vanish
// from the render rather than reporting anything, so a typo in a scene file would
// be near-impossible to trace.
func QuatFromAxisAngle(axisAngle mgl32.Vec4) mgl32.Quat {
	axis := axisAngle.Vec3()
	if axis.Len() == 0 {
		axis = defaultAxis
	}
	return mgl32.QuatRotate(mgl32.DegToRad(axisAngle.W()), axis.Normalize())
}

// AxisAngleFromQuat converts a rotation back to XYZ axis plus degrees, for the
// scene file, the wire and the inspector.
//
// It is the inverse of QuatFromAxisAngle as a *rotation*, not as text: a
// quaternion has no memory of which of the several axis-angle pairs naming the
// same rotation was written down. A negative angle comes back positive with the
// sign moved onto the axis, so [0, 1, 0, -45] becomes [0, -1, 0, 45]. What
// matters is that repeating the conversion does not keep changing the answer,
// because a scene gets loaded and saved over and over.
//
// The arithmetic is float64 and uses atan2 rather than acos. acos is
// ill-conditioned near an angle of zero, which is exactly where every unrotated
// object in a scene sits, and atan2 of the vector length against the scalar part
// stays accurate across the whole range.
func AxisAngleFromQuat(q mgl32.Quat) mgl32.Vec4 {
	if q.Len() == 0 {
		return identityAxisAngle()
	}
	q = q.Normalize()

	w := float64(q.W)
	x, y, z := float64(q.V[0]), float64(q.V[1]), float64(q.V[2])

	length := math.Sqrt(x*x + y*y + z*z)
	if length < negligibleRotation {
		// No rotation to speak of, so the axis is arbitrary and whatever is left
		// in the vector part is float noise. This covers a full turn as well as
		// no turn at all: at 360 degrees the quaternion is the negated identity,
		// and float32 leaves about 9e-8 in the vector part, enough for the axis
		// to come out as an arbitrary sign if it were divided through.
		return identityAxisAngle()
	}

	// atan2 of the vector length against the scalar part, rather than acos of the
	// scalar part alone. Three things fall out of it:
	//
	// The angle spans the whole [0, 360) turn instead of being folded into
	// [0, 180] with the sign pushed onto the axis. A spinning entity therefore
	// reports an angle that climbs the way its component counts it, rather than
	// reaching 180 and coming back down with a flipped axis — which is what a
	// person watching the inspector, or a gRPC client, would see otherwise.
	//
	// It is continuous through a half turn. The scalar part is zero there and
	// float32 puts it a hair either side, so anything keying off its sign made a
	// 180 degree rotation alternate between an axis and its opposite on
	// successive saves. atan2 does not care which side of zero it is on.
	//
	// And dividing the axis by its own measured length, rather than by
	// sqrt(1-w*w), leaves it exactly unit. Deriving the length from w instead
	// left it a few ulps short, so the first save of a scene disagreed with every
	// save after it.
	degrees := 2 * math.Atan2(length, w) * 180 / math.Pi

	return mgl32.Vec4{
		float32(x / length),
		float32(y / length),
		float32(z / length),
		float32(degrees),
	}
}

func identityAxisAngle() mgl32.Vec4 {
	return mgl32.Vec4{defaultAxis.X(), defaultAxis.Y(), defaultAxis.Z(), 0}
}

// negligibleRotation is the vector-part length below which a quaternion is
// treated as no rotation at all. It corresponds to about a ten-thousandth of a
// degree, comfortably above the float32 noise left at a whole turn and far below
// anything a scene would mean to express.
const negligibleRotation = 1e-6
