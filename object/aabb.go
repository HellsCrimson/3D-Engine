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

// Separation returns the smallest translation that pushes b clear of other,
// along a single axis. It is the zero vector when the boxes do not overlap.
//
// Each axis offers two ways out, and the distance is measured to the face being
// left rather than from the width of the overlap:
//
//	toNegative = b.Max - other.Min   // far enough to put b's max on other's min
//	toPositive = other.Max - b.Min   // far enough to put b's min on other's max
//
// Taking the width of the intersection instead — min of the maxima less max of
// the minima — is right only while the boxes straddle each other. Once one is
// wholly inside the other on an axis, that width is the inner box's own extent,
// which is generally too small to reach either face, and a deeply embedded body
// creeps out over many steps instead of popping out in one.
//
// The two agree wherever the old formula was correct, so this changes behaviour
// only for the containment case it was getting wrong.
func (b AABB) Separation(other AABB) mgl32.Vec3 {
	var best mgl32.Vec3
	var bestDistance float32

	for axis := 0; axis < 3; axis++ {
		toNegative := b.Max[axis] - other.Min[axis]
		toPositive := other.Max[axis] - b.Min[axis]

		// Both positive is exactly the condition for overlapping on this axis,
		// so one non-positive means the boxes are apart and there is nothing to
		// resolve.
		if toNegative <= 0 || toPositive <= 0 {
			return mgl32.Vec3{0, 0, 0}
		}

		distance, direction := toNegative, float32(-1)
		if toPositive < toNegative {
			distance, direction = toPositive, 1
		}

		// Strictly less, so the earliest axis wins a tie — the order the old
		// chain of comparisons resolved them in.
		if axis == 0 || distance < bestDistance {
			bestDistance = distance
			best = mgl32.Vec3{}
			best[axis] = distance * direction
		}
	}

	return best
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
