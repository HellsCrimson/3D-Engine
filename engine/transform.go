package engine

import "github.com/go-gl/mathgl/mgl32"

// Transform is a local placement. Rotation stays in the axis-angle form the
// scene files and the RPC surface already use — XYZ is the axis, W the angle in
// degrees — rather than a quaternion, so neither format has to change here.
type Transform struct {
	Position mgl32.Vec3
	Rotation mgl32.Vec4
	Scale    mgl32.Vec3
}

// IdentityTransform is the sane default for an entity created in code: unit
// scale rather than the zero scale a bare Transform{} would give.
func IdentityTransform() Transform {
	return Transform{
		Rotation: mgl32.Vec4{0, 1, 0, 0},
		Scale:    mgl32.Vec3{1, 1, 1},
	}
}

// Matrix builds the translate -> rotate -> scale matrix. This used to be
// open-coded in three places (the render loop, the player loop and
// Model.worldTransform); it lives here now so the order can only be changed
// once.
func (t Transform) Matrix() mgl32.Mat4 {
	mat := mgl32.Translate3D(t.Position.X(), t.Position.Y(), t.Position.Z())
	mat = mat.Mul4(mgl32.HomogRotate3D(mgl32.DegToRad(t.Rotation.W()), t.Rotation.Vec3()))
	return mat.Mul4(mgl32.Scale3D(t.Scale.X(), t.Scale.Y(), t.Scale.Z()))
}
