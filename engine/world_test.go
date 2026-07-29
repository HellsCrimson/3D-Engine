package engine

import (
	"testing"

	"github.com/go-gl/mathgl/mgl32"
)

func TestHandleEncodeRoundTrip(t *testing.T) {
	cases := []Handle{
		{Index: 0, Generation: 1},
		{Index: 1, Generation: 1},
		{Index: 4294967295, Generation: 4294967295},
		{Index: 7, Generation: 900},
	}

	for _, want := range cases {
		if got := DecodeHandle(want.Encode()); got != want {
			t.Fatalf("round trip: got %v, want %v", got, want)
		}
	}

	if !DecodeHandle(0).IsZero() {
		t.Fatal("zero encoding should decode to the nil handle")
	}
}

func TestSpawnedHandleIsNeverZero(t *testing.T) {
	w := NewWorld()
	e := w.Spawn(NewEntity("first"))

	if e.Handle().IsZero() {
		t.Fatal("a spawned entity must have a usable handle")
	}
	if w.Get(e.Handle()) != e {
		t.Fatal("handle did not resolve back to its entity")
	}
	if w.Get(NoHandle) != nil {
		t.Fatal("the nil handle must not resolve")
	}
}

// TestDespawnedHandleGoesStale is the whole point of the generation counter:
// the slot gets reused, and the old handle must not follow it.
func TestDespawnedHandleGoesStale(t *testing.T) {
	w := NewWorld()

	old := w.Spawn(NewEntity("old"))
	oldHandle := old.Handle()

	if !w.Despawn(oldHandle) {
		t.Fatal("despawn of a live handle should succeed")
	}
	if w.Get(oldHandle) != nil {
		t.Fatal("despawned handle still resolves")
	}
	if w.Despawn(oldHandle) {
		t.Fatal("despawning twice should report the handle as dead")
	}

	// The next spawn reuses the freed slot.
	fresh := w.Spawn(NewEntity("fresh"))
	if fresh.Handle().Index != oldHandle.Index {
		t.Fatalf("expected slot reuse: fresh index %d, old index %d",
			fresh.Handle().Index, oldHandle.Index)
	}
	if fresh.Handle().Generation == oldHandle.Generation {
		t.Fatal("reused slot kept its generation, so stale handles would alias")
	}

	if got := w.Get(oldHandle); got != nil {
		t.Fatalf("stale handle resolved to the new occupant %q", got.Name)
	}
	if w.Get(fresh.Handle()) != fresh {
		t.Fatal("fresh handle should resolve")
	}
}

// TestReplaceInvalidatesOldHandles covers the reported bug: ids used to restart
// from zero on every scene load, so a UI holding them addressed the wrong
// object after a scene switch.
func TestReplaceInvalidatesOldHandles(t *testing.T) {
	w := NewWorld()

	first := w.Spawn(NewEntity("scene-one-object"))
	staleHandle := first.Handle()

	w.Replace([]*Entity{NewEntity("scene-two-object")})

	if got := w.Get(staleHandle); got != nil {
		t.Fatalf("handle from the previous scene resolved to %q", got.Name)
	}
	if w.Mutate(staleHandle, func(*Entity) {}) {
		t.Fatal("mutating through a stale handle should fail, not hit a new object")
	}

	replacement := w.Find("scene-two-object")
	if replacement == nil {
		t.Fatal("replacement entity missing")
	}
	if w.Get(replacement.Handle()) != replacement {
		t.Fatal("replacement handle should resolve")
	}
}

// TestDespawnKeepsRemainingEntitiesAddressable guards the swap-remove: moving
// the last entity into the freed position must update its slot mapping.
func TestDespawnKeepsRemainingEntitiesAddressable(t *testing.T) {
	w := NewWorld()

	names := []string{"a", "b", "c", "d", "e"}
	handles := map[string]Handle{}
	for _, name := range names {
		handles[name] = w.Spawn(NewEntity(name)).Handle()
	}

	// Remove from the middle so a swap actually happens.
	if !w.Despawn(handles["b"]) {
		t.Fatal("despawn failed")
	}
	if !w.Despawn(handles["d"]) {
		t.Fatal("despawn failed")
	}

	if w.Len() != 3 {
		t.Fatalf("expected 3 entities left, got %d", w.Len())
	}

	for _, name := range []string{"a", "c", "e"} {
		entity := w.Get(handles[name])
		if entity == nil {
			t.Fatalf("%q became unaddressable after unrelated despawns", name)
		}
		if entity.Name != name {
			t.Fatalf("handle for %q now resolves to %q", name, entity.Name)
		}
	}

	seen := map[string]bool{}
	w.Read(func(entities []*Entity) {
		for _, entity := range entities {
			if entity == nil {
				t.Fatal("nil hole left in the dense entity slice")
			}
			seen[entity.Name] = true
		}
	})
	if len(seen) != 3 || !seen["a"] || !seen["c"] || !seen["e"] {
		t.Fatalf("iteration returned %v", seen)
	}
}

// TestParentInvalidationPropagates checks the cached world matrix: moving a
// parent must dirty its descendants.
func TestParentInvalidationPropagates(t *testing.T) {
	parent := NewEntity("parent")
	child := NewEntity("child")
	grandchild := NewEntity("grandchild")

	child.SetParent(parent)
	grandchild.SetParent(child)

	parent.SetPosition(mgl3(1, 0, 0))
	child.SetPosition(mgl3(0, 2, 0))
	grandchild.SetPosition(mgl3(0, 0, 3))

	// Prime the caches.
	_ = grandchild.WorldMatrix()

	parent.SetPosition(mgl3(10, 0, 0))

	got := grandchild.WorldMatrix().Col(3).Vec3()
	want := mgl3(10, 2, 3)
	for axis := 0; axis < 3; axis++ {
		if diff := got[axis] - want[axis]; diff > 1e-5 || diff < -1e-5 {
			t.Fatalf("stale cached world matrix: got %v, want %v", got, want)
		}
	}
}

func mgl3(x, y, z float32) mgl32.Vec3 {
	return mgl32.Vec3{x, y, z}
}
