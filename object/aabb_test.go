package object

import (
	"math/rand"
	"testing"

	"github.com/go-gl/mathgl/mgl32"
)

// legacySeparation is the algorithm as it was written before AABB existed. It
// was duplicated verbatim in Model.CollisionSeparation and in
// App.playerAABBCollisionSeparation; both now call AABB.Separation, so this
// pins the consolidation to the original behaviour.
func legacySeparation(aMin, aMax, bMin, bMax mgl32.Vec3) mgl32.Vec3 {
	overlapX := minf(aMax.X(), bMax.X()) - maxf(aMin.X(), bMin.X())
	overlapY := minf(aMax.Y(), bMax.Y()) - maxf(aMin.Y(), bMin.Y())
	overlapZ := minf(aMax.Z(), bMax.Z()) - maxf(aMin.Z(), bMin.Z())
	if overlapX <= 0 || overlapY <= 0 || overlapZ <= 0 {
		return mgl32.Vec3{0, 0, 0}
	}

	centerA := aMin.Add(aMax).Mul(0.5)
	centerB := bMin.Add(bMax).Mul(0.5)

	if overlapX <= overlapY && overlapX <= overlapZ {
		if centerA.X() < centerB.X() {
			return mgl32.Vec3{-overlapX, 0, 0}
		}
		return mgl32.Vec3{overlapX, 0, 0}
	}

	if overlapY <= overlapX && overlapY <= overlapZ {
		if centerA.Y() < centerB.Y() {
			return mgl32.Vec3{0, -overlapY, 0}
		}
		return mgl32.Vec3{0, overlapY, 0}
	}

	if centerA.Z() < centerB.Z() {
		return mgl32.Vec3{0, 0, -overlapZ}
	}
	return mgl32.Vec3{0, 0, overlapZ}
}

func randomBox(rng *rand.Rand) AABB {
	origin := mgl32.Vec3{
		rng.Float32()*10 - 5,
		rng.Float32()*10 - 5,
		rng.Float32()*10 - 5,
	}
	size := mgl32.Vec3{
		rng.Float32() * 4,
		rng.Float32() * 4,
		rng.Float32() * 4,
	}
	return AABB{Min: origin, Max: origin.Add(size)}
}

func TestSeparationMatchesLegacy(t *testing.T) {
	rng := rand.New(rand.NewSource(1))

	overlapping := 0
	for i := 0; i < 20000; i++ {
		a := randomBox(rng)
		b := randomBox(rng)

		got := a.Separation(b)
		want := legacySeparation(a.Min, a.Max, b.Min, b.Max)
		if got != want {
			t.Fatalf("case %d: a=%v b=%v: got %v, legacy %v", i, a, b, got, want)
		}
		if got != (mgl32.Vec3{}) {
			overlapping++
		}
	}

	if overlapping == 0 {
		t.Fatal("no overlapping pairs generated, the comparison proved nothing")
	}
	t.Logf("%d of 20000 pairs overlapped", overlapping)
}

// containedOnSomeAxis reports whether either box's extent sits entirely inside
// the other's on any axis. See TestSeparationUnderSeparatesWhenContained.
func containedOnSomeAxis(a, b AABB) bool {
	for axis := 0; axis < 3; axis++ {
		aInB := a.Min[axis] >= b.Min[axis] && a.Max[axis] <= b.Max[axis]
		bInA := b.Min[axis] >= a.Min[axis] && b.Max[axis] <= a.Max[axis]
		if aInB || bInA {
			return true
		}
	}
	return false
}

// TestSeparationResolvesOverlap checks the contract the physics loop relies on:
// applying the separation to the moving box leaves the two no longer
// penetrating. Boxes where one is contained in the other on some axis are
// excluded — see TestSeparationUnderSeparatesWhenContained.
func TestSeparationResolvesOverlap(t *testing.T) {
	rng := rand.New(rand.NewSource(2))

	checked := 0
	for i := 0; i < 20000; i++ {
		a := randomBox(rng)
		b := randomBox(rng)
		if !a.Intersects(b) || containedOnSomeAxis(a, b) {
			continue
		}

		separation := a.Separation(b)
		if separation == (mgl32.Vec3{}) {
			// Touching exactly at a face counts as intersecting but has no
			// penetration to resolve.
			continue
		}
		checked++

		moved := AABB{Min: a.Min.Add(separation), Max: a.Max.Add(separation)}
		overlapX := minf(moved.Max.X(), b.Max.X()) - maxf(moved.Min.X(), b.Min.X())
		overlapY := minf(moved.Max.Y(), b.Max.Y()) - maxf(moved.Min.Y(), b.Min.Y())
		overlapZ := minf(moved.Max.Z(), b.Max.Z()) - maxf(moved.Min.Z(), b.Min.Z())

		const epsilon = 1e-4
		if overlapX > epsilon && overlapY > epsilon && overlapZ > epsilon {
			t.Fatalf("case %d: still penetrating after separation %v: a=%v b=%v", i, separation, a, b)
		}
	}

	if checked == 0 {
		t.Fatal("no cases exercised, the assertion proved nothing")
	}
	t.Logf("%d separations resolved cleanly", checked)
}

// TestSeparationUnderSeparatesWhenContained documents a limitation that predates
// the AABB consolidation: when one box sits entirely inside the other along the
// minimum-overlap axis, the overlap formula yields the contained box's whole
// extent instead of the distance to the nearest face, so a single step does not
// push far enough to clear the other box. The physics loop hides this by
// re-running every fixed step, but a deeply embedded body creeps out rather than
// popping out.
//
// This is pinned rather than fixed so that a future change to the separation
// algorithm is a deliberate decision instead of an accident.
func TestSeparationUnderSeparatesWhenContained(t *testing.T) {
	// A's Z extent lies wholly within B's; Z is the minimum-overlap axis.
	a := AABB{
		Min: mgl32.Vec3{0.75190973, -2.0266662, -1.8225768},
		Max: mgl32.Vec3{1.6058164, 1.3054953, -1.4170029},
	}
	b := AABB{
		Min: mgl32.Vec3{0.96693707, -1.1084576, -2.023699},
		Max: mgl32.Vec3{2.0310278, 0.8411696, -0.20846915},
	}

	separation := a.Separation(b)
	if separation.X() != 0 || separation.Y() != 0 || separation.Z() >= 0 {
		t.Fatalf("expected a push along -Z, got %v", separation)
	}

	// Clearing B along -Z needs b.Min.Z - a.Max.Z; the algorithm gives less.
	needed := b.Min.Z() - a.Max.Z()
	if separation.Z() <= needed {
		t.Fatalf("separation %v already clears the box (needed %v); "+
			"the containment limitation appears to be fixed, so update this test",
			separation.Z(), needed)
	}

	moved := AABB{Min: a.Min.Add(separation), Max: a.Max.Add(separation)}
	if !moved.Intersects(b) {
		t.Fatal("expected the boxes to still overlap after one separation step")
	}
}

// TestTransformFitsRotatedCorners pins the conservative re-fit: every
// transformed corner of the source box must land inside the result.
func TestTransformFitsRotatedCorners(t *testing.T) {
	box := AABB{Min: mgl32.Vec3{-1, -2, -3}, Max: mgl32.Vec3{4, 5, 6}}

	mat := mgl32.Translate3D(3, -2, 7).
		Mul4(mgl32.HomogRotate3D(mgl32.DegToRad(37), mgl32.Vec3{0.3, 0.8, 0.5}.Normalize())).
		Mul4(mgl32.Scale3D(2, 0.5, 1.5))

	got := box.Transform(mat)

	corners := [8]mgl32.Vec3{
		{box.Min.X(), box.Min.Y(), box.Min.Z()},
		{box.Max.X(), box.Min.Y(), box.Min.Z()},
		{box.Min.X(), box.Max.Y(), box.Min.Z()},
		{box.Max.X(), box.Max.Y(), box.Min.Z()},
		{box.Min.X(), box.Min.Y(), box.Max.Z()},
		{box.Max.X(), box.Min.Y(), box.Max.Z()},
		{box.Min.X(), box.Max.Y(), box.Max.Z()},
		{box.Max.X(), box.Max.Y(), box.Max.Z()},
	}

	const epsilon = 1e-4
	for i, corner := range corners {
		world := mat.Mul4x1(corner.Vec4(1)).Vec3()
		for axis := 0; axis < 3; axis++ {
			if world[axis] < got.Min[axis]-epsilon || world[axis] > got.Max[axis]+epsilon {
				t.Fatalf("corner %d axis %d: %v outside %v..%v", i, axis, world[axis], got.Min[axis], got.Max[axis])
			}
		}
	}

	// The fit must be tight: each bound is touched by some corner.
	for axis := 0; axis < 3; axis++ {
		touchedMin, touchedMax := false, false
		for _, corner := range corners {
			world := mat.Mul4x1(corner.Vec4(1)).Vec3()
			if world[axis] <= got.Min[axis]+epsilon {
				touchedMin = true
			}
			if world[axis] >= got.Max[axis]-epsilon {
				touchedMax = true
			}
		}
		if !touchedMin || !touchedMax {
			t.Fatalf("axis %d bounds are looser than the corner fit", axis)
		}
	}
}

// TestIdentityTransformIsStable guards the common case: an unrotated,
// unscaled, untranslated entity must not have its bounds drift.
func TestIdentityTransformIsStable(t *testing.T) {
	box := AABB{Min: mgl32.Vec3{-1, -2, -3}, Max: mgl32.Vec3{4, 5, 6}}
	if got := box.Transform(mgl32.Ident4()); got != box {
		t.Fatalf("identity transform changed the box: %v -> %v", box, got)
	}
}
