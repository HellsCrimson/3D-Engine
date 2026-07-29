package object

import "github.com/go-gl/mathgl/mgl32"

// AABB is an axis-aligned bounding box in whatever space the caller is working
// in. Model, Mesh and the player capsule all produce one, which is what lets
// them share the collision math below.
type AABB struct {
	Min mgl32.Vec3
	Max mgl32.Vec3
}

// PointAABB is the degenerate box used for geometry with no vertices.
func PointAABB(p mgl32.Vec3) AABB {
	return AABB{Min: p, Max: p}
}

func (b AABB) Center() mgl32.Vec3 {
	return b.Min.Add(b.Max).Mul(0.5)
}

// Transform re-fits an axis-aligned box around the eight transformed corners.
// A rotated box therefore gets a conservative — never too small — bound.
func (b AABB) Transform(mat mgl32.Mat4) AABB {
	corners := [8]mgl32.Vec3{
		{b.Min.X(), b.Min.Y(), b.Min.Z()},
		{b.Max.X(), b.Min.Y(), b.Min.Z()},
		{b.Min.X(), b.Max.Y(), b.Min.Z()},
		{b.Max.X(), b.Max.Y(), b.Min.Z()},
		{b.Min.X(), b.Min.Y(), b.Max.Z()},
		{b.Max.X(), b.Min.Y(), b.Max.Z()},
		{b.Min.X(), b.Max.Y(), b.Max.Z()},
		{b.Max.X(), b.Max.Y(), b.Max.Z()},
	}

	first := mat.Mul4x1(corners[0].Vec4(1.0)).Vec3()
	out := AABB{Min: first, Max: first}

	for i := 1; i < len(corners); i++ {
		corner := mat.Mul4x1(corners[i].Vec4(1.0)).Vec3()
		for axis := 0; axis < 3; axis++ {
			if corner[axis] < out.Min[axis] {
				out.Min[axis] = corner[axis]
			}
			if corner[axis] > out.Max[axis] {
				out.Max[axis] = corner[axis]
			}
		}
	}

	return out
}

func (b AABB) Intersects(other AABB) bool {
	return b.Min.X() <= other.Max.X() && b.Max.X() >= other.Min.X() &&
		b.Min.Y() <= other.Max.Y() && b.Max.Y() >= other.Min.Y() &&
		b.Min.Z() <= other.Max.Z() && b.Max.Z() >= other.Min.Z()
}

// Separation returns the smallest translation that pushes b out of other, along
// whichever single axis overlaps least. It is the zero vector when the boxes do
// not overlap.
func (b AABB) Separation(other AABB) mgl32.Vec3 {
	overlapX := minf(b.Max.X(), other.Max.X()) - maxf(b.Min.X(), other.Min.X())
	overlapY := minf(b.Max.Y(), other.Max.Y()) - maxf(b.Min.Y(), other.Min.Y())
	overlapZ := minf(b.Max.Z(), other.Max.Z()) - maxf(b.Min.Z(), other.Min.Z())
	if overlapX <= 0 || overlapY <= 0 || overlapZ <= 0 {
		return mgl32.Vec3{0, 0, 0}
	}

	center := b.Center()
	otherCenter := other.Center()

	if overlapX <= overlapY && overlapX <= overlapZ {
		if center.X() < otherCenter.X() {
			return mgl32.Vec3{-overlapX, 0, 0}
		}
		return mgl32.Vec3{overlapX, 0, 0}
	}

	if overlapY <= overlapX && overlapY <= overlapZ {
		if center.Y() < otherCenter.Y() {
			return mgl32.Vec3{0, -overlapY, 0}
		}
		return mgl32.Vec3{0, overlapY, 0}
	}

	if center.Z() < otherCenter.Z() {
		return mgl32.Vec3{0, 0, -overlapZ}
	}
	return mgl32.Vec3{0, 0, overlapZ}
}

func minf(a, b float32) float32 {
	if a < b {
		return a
	}
	return b
}

func maxf(a, b float32) float32 {
	if a > b {
		return a
	}
	return b
}
