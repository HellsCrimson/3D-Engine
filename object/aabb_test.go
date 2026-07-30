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

// TestSeparationMatchesLegacyWhereLegacyWasRight keeps the consolidation pinned
// everywhere the original algorithm was correct, which is every pair where
// neither box is swallowed by the other on any axis. Containment is the one case
// the two deliberately disagree on, and
// TestSeparationClearsAContainedBox covers that side.
func TestSeparationMatchesLegacyWhereLegacyWasRight(t *testing.T) {
	rng := rand.New(rand.NewSource(1))

	compared := 0
	for i := 0; i < 20000; i++ {
		a := randomBox(rng)
		b := randomBox(rng)
		if containedOnSomeAxis(a, b) {
			continue
		}

		got := a.Separation(b)
		want := legacySeparation(a.Min, a.Max, b.Min, b.Max)
		if got != want {
			t.Fatalf("case %d: a=%v b=%v: got %v, legacy %v", i, a, b, got, want)
		}
		if got != (mgl32.Vec3{}) {
			compared++
		}
	}

	if compared == 0 {
		t.Fatal("no overlapping pairs generated, the comparison proved nothing")
	}
	t.Logf("%d of 20000 pairs overlapped and agreed with the original", compared)
}

// TestSeparationBeatsLegacyOnContainment is the other half: where the two
// disagree, the new one has to be the one that actually resolves the overlap.
// Without this the test above could be satisfied by any change that merely avoids
// the containment cases.
func TestSeparationBeatsLegacyOnContainment(t *testing.T) {
	rng := rand.New(rand.NewSource(7))

	disagreements := 0
	for i := 0; i < 40000; i++ {
		a := randomBox(rng)
		b := randomBox(rng)
		if !a.Intersects(b) || !containedOnSomeAxis(a, b) {
			continue
		}

		got := a.Separation(b)
		legacy := legacySeparation(a.Min, a.Max, b.Min, b.Max)
		if got == legacy {
			continue
		}
		disagreements++

		// The old answer left them touching or overlapping; the new one must not.
		movedByLegacy := AABB{Min: a.Min.Add(legacy), Max: a.Max.Add(legacy)}
		moved := AABB{Min: a.Min.Add(got), Max: a.Max.Add(got)}

		if stillPenetrating(moved, b) {
			t.Fatalf("case %d: still penetrating after %v: a=%v b=%v", i, got, a, b)
		}
		if !stillPenetrating(movedByLegacy, b) {
			t.Fatalf("case %d: the original answer %v was fine, so this pair does not "+
				"belong in this test: a=%v b=%v", i, legacy, a, b)
		}
	}

	if disagreements == 0 {
		t.Fatal("no containment disagreements generated, the assertion proved nothing")
	}
	t.Logf("%d containment cases the original algorithm under-separated", disagreements)
}

// stillPenetrating reports whether two boxes overlap by more than rounding on
// every axis. Touching exactly at a face is not penetration.
func stillPenetrating(a, b AABB) bool {
	const epsilon = 1e-4

	for axis := 0; axis < 3; axis++ {
		overlap := minf(a.Max[axis], b.Max[axis]) - maxf(a.Min[axis], b.Min[axis])
		if overlap <= epsilon {
			return false
		}
	}
	return true
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
// penetrating.
//
// Contained boxes used to be excluded here because the algorithm could not
// handle them. They are included now, which is the whole point of the fix.
func TestSeparationResolvesOverlap(t *testing.T) {
	rng := rand.New(rand.NewSource(2))

	checked, contained := 0, 0
	for i := 0; i < 20000; i++ {
		a := randomBox(rng)
		b := randomBox(rng)
		if !a.Intersects(b) {
			continue
		}

		separation := a.Separation(b)
		if separation == (mgl32.Vec3{}) {
			// Touching exactly at a face counts as intersecting but has no
			// penetration to resolve.
			continue
		}
		checked++
		if containedOnSomeAxis(a, b) {
			contained++
		}

		moved := AABB{Min: a.Min.Add(separation), Max: a.Max.Add(separation)}
		if stillPenetrating(moved, b) {
			t.Fatalf("case %d: still penetrating after separation %v: a=%v b=%v", i, separation, a, b)
		}
	}

	if checked == 0 {
		t.Fatal("no cases exercised, the assertion proved nothing")
	}
	if contained == 0 {
		t.Fatal("no contained pairs exercised, so the case this fix was for went untested")
	}
	t.Logf("%d separations resolved cleanly, %d of them contained on some axis", checked, contained)
}

// TestSeparationClearsAContainedBox is the fixture that used to pin the
// containment bug, now asserting the opposite.
//
// A's Z extent lies wholly within B's. The old algorithm measured the overlap as
// A's own Z extent, which was not far enough to reach either of B's faces, so a
// body embedded this deeply crept out over many fixed steps instead of popping
// out in one. It now moves exactly as far as the nearer face.
func TestSeparationClearsAContainedBox(t *testing.T) {
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

	// -Z is the shorter way out here, and clearing B that way needs exactly
	// b.Min.Z - a.Max.Z.
	needed := b.Min.Z() - a.Max.Z()
	if separation.Z() > needed {
		t.Fatalf("separation %v does not clear the box, needed %v", separation.Z(), needed)
	}

	moved := AABB{Min: a.Min.Add(separation), Max: a.Max.Add(separation)}
	if stillPenetrating(moved, b) {
		t.Fatalf("the boxes still overlap after one separation step: %v vs %v", moved, b)
	}

	// One step, not several: it must not overshoot either.
	if separation.Z() < needed-1e-4 {
		t.Errorf("separation %v overshoots; the nearest face is at %v", separation.Z(), needed)
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
