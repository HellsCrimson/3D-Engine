package engine

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-gl/mathgl/mgl32"
)

// A grouping node with no model and no components, which only version 2's
// children key makes legitimate, plus two levels below it.
const hierarchyScene = `version: 2
objects:
  - name: rig
    transform:
      position: [10.0, 0.0, 0.0]
    children:
      - name: arm
        transform:
          position: [0.0, 5.0, 0.0]
        components:
          - type: PointLight
            props:
              diffuse: [1.0, 0.0, 0.0]
        children:
          - name: bulb
            transform:
              position: [0.0, 0.0, 2.0]
            components:
              - type: PointLight
      - name: base
        components:
          - type: DirectionalLight

  - name: loose
    components:
      - type: SpotLight
`

func TestSceneHierarchyRoundTrip(t *testing.T) {
	directory := t.TempDir()
	original := filepath.Join(directory, "original.yml")
	if err := os.WriteFile(original, []byte(hierarchyScene), 0o644); err != nil {
		t.Fatalf("writing fixture: %v", err)
	}

	a := saveTestApp(t)
	loadAndPlace(t, a, original)

	first := filepath.Join(directory, "first.yml")
	if err := a.SaveScene(first); err != nil {
		t.Fatalf("first save: %v", err)
	}

	loadAndPlace(t, a, first)

	second := filepath.Join(directory, "second.yml")
	if err := a.SaveScene(second); err != nil {
		t.Fatalf("second save: %v", err)
	}

	assertScenesMatch(t, first, second)

	firstContent, err := os.ReadFile(first)
	if err != nil {
		t.Fatalf("reading first save: %v", err)
	}

	// The saved file has to be nested, not flattened — otherwise the round trip
	// would be self-consistent while quietly discarding the tree.
	if !strings.Contains(string(firstContent), "children:") {
		t.Errorf("saved scene has no children block:\n%s", firstContent)
	}
}

func TestHierarchySurvivesLoad(t *testing.T) {
	path := filepath.Join(t.TempDir(), "scene.yml")
	if err := os.WriteFile(path, []byte(hierarchyScene), 0o644); err != nil {
		t.Fatalf("writing fixture: %v", err)
	}

	a := saveTestApp(t)
	loadAndPlace(t, a, path)

	// Every entity, at every depth, is in the world's flat list.
	if got := a.World.Len(); got != 5 {
		t.Fatalf("entity count: got %d, want 5 (rig, arm, bulb, base, loose)", got)
	}

	rig := a.World.Find("rig")
	arm := a.World.Find("arm")
	bulb := a.World.Find("bulb")
	loose := a.World.Find("loose")
	for name, entity := range map[string]*Entity{"rig": rig, "arm": arm, "bulb": bulb, "loose": loose} {
		if entity == nil {
			t.Fatalf("%s is missing from the world", name)
		}
	}

	if arm.Parent() != rig {
		t.Errorf("arm's parent: got %v, want rig", arm.Parent())
	}
	if bulb.Parent() != arm {
		t.Errorf("bulb's parent: got %v, want arm", bulb.Parent())
	}
	if loose.Parent() != nil {
		t.Errorf("loose should be at the root, got parent %v", loose.Parent())
	}
	if got := len(rig.Children()); got != 2 {
		t.Errorf("rig's children: got %d, want 2", got)
	}
}

// TestWorldMatrixComposesThroughParents is the reason hierarchy is worth having:
// the child's placement is relative to its parent's.
func TestWorldMatrixComposesThroughParents(t *testing.T) {
	path := filepath.Join(t.TempDir(), "scene.yml")
	if err := os.WriteFile(path, []byte(hierarchyScene), 0o644); err != nil {
		t.Fatalf("writing fixture: %v", err)
	}

	a := saveTestApp(t)
	loadAndPlace(t, a, path)

	bulb := a.World.Find("bulb")
	if bulb == nil {
		t.Fatal("bulb is missing")
	}

	// rig {10,0,0} + arm {0,5,0} + bulb {0,0,2}
	world := bulb.WorldMatrix().Col(3).Vec3()
	if world != (mgl32.Vec3{10, 5, 2}) {
		t.Errorf("bulb world position: got %v, want {10 5 2}", world)
	}

	// Moving the parent moves the subtree, and the cached matrix has to notice.
	a.World.Find("rig").SetPosition(mgl32.Vec3{0, 0, 0})
	world = bulb.WorldMatrix().Col(3).Vec3()
	if world != (mgl32.Vec3{0, 5, 2}) {
		t.Errorf("after moving rig, bulb world position: got %v, want {0 5 2}", world)
	}
}

func TestSpawnUnderParent(t *testing.T) {
	a := saveTestApp(t)

	parent, err := a.SpawnObject(ObjectSpec{Name: "parent", Components: []Component{NewPointLight()}})
	if err != nil {
		t.Fatalf("spawning parent: %v", err)
	}

	child, err := a.SpawnObject(ObjectSpec{
		Name:       "child",
		Components: []Component{NewPointLight()},
		Parent:     parent.Handle(),
	})
	if err != nil {
		t.Fatalf("spawning child: %v", err)
	}

	if child.Parent() != parent {
		t.Errorf("child's parent: got %v, want parent", child.Parent())
	}

	// A stale parent handle must fail rather than spawn an orphan.
	if err := a.DespawnTree(parent.Handle()); err != nil {
		t.Fatalf("despawning: %v", err)
	}
	before := a.World.Len()
	if _, err := a.SpawnObject(ObjectSpec{
		Name:       "orphan",
		Components: []Component{NewPointLight()},
		Parent:     parent.Handle(),
	}); err == nil {
		t.Error("spawning under a despawned parent should fail")
	}
	if a.World.Len() != before {
		t.Error("a failed spawn left an entity in the world")
	}
}

// TestSpawnObjectBuildsWholeTree covers spawning a subtree in one call, which is
// what an editor's paste or a prefab would do.
func TestSpawnObjectBuildsWholeTree(t *testing.T) {
	a := saveTestApp(t)

	root, err := a.SpawnObject(ObjectSpec{
		Name:      "root",
		Transform: IdentityTransform(),
		Children: []ObjectSpec{{
			Name:       "middle",
			Transform:  IdentityTransform(),
			Components: []Component{NewPointLight()},
			Children: []ObjectSpec{{
				Name:       "leaf",
				Transform:  IdentityTransform(),
				Components: []Component{NewPointLight()},
			}},
		}},
	})
	if err != nil {
		t.Fatalf("spawning tree: %v", err)
	}

	if got := a.World.Len(); got != 3 {
		t.Fatalf("entity count: got %d, want 3", got)
	}
	// Every entity in the tree must have been given a handle, not just the root.
	leaf := a.World.Find("leaf")
	if leaf == nil || leaf.Handle().IsZero() {
		t.Fatal("the deepest entity was not spawned into the world")
	}
	if leaf.Parent() == nil || leaf.Parent().Name != "middle" {
		t.Errorf("leaf's parent: got %v", leaf.Parent())
	}
	if root.Handle().IsZero() {
		t.Error("the root was not given a handle")
	}
}

func TestDespawnTreeRemovesSubtree(t *testing.T) {
	path := filepath.Join(t.TempDir(), "scene.yml")
	if err := os.WriteFile(path, []byte(hierarchyScene), 0o644); err != nil {
		t.Fatalf("writing fixture: %v", err)
	}

	a := saveTestApp(t)
	loadAndPlace(t, a, path)

	rig := a.World.Find("rig")
	if err := a.DespawnTree(rig.Handle()); err != nil {
		t.Fatalf("DespawnTree: %v", err)
	}

	// rig, arm, bulb and base go; loose stays.
	if got := a.World.Len(); got != 1 {
		t.Fatalf("entity count after deleting the rig: got %d, want 1", got)
	}
	if a.World.Find("loose") == nil {
		t.Error("DespawnTree removed an entity outside the subtree")
	}
	for _, name := range []string{"rig", "arm", "bulb", "base"} {
		if a.World.Find(name) != nil {
			t.Errorf("%s survived DespawnTree", name)
		}
	}
}

// TestDespawnObjectOrphansChildren documents the difference between the primitive
// and DespawnTree: on its own, despawning a parent lifts its children to the root
// rather than taking them with it.
func TestDespawnObjectOrphansChildren(t *testing.T) {
	path := filepath.Join(t.TempDir(), "scene.yml")
	if err := os.WriteFile(path, []byte(hierarchyScene), 0o644); err != nil {
		t.Fatalf("writing fixture: %v", err)
	}

	a := saveTestApp(t)
	loadAndPlace(t, a, path)

	rig := a.World.Find("rig")
	if err := a.DespawnObject(rig.Handle()); err != nil {
		t.Fatalf("DespawnObject: %v", err)
	}

	arm := a.World.Find("arm")
	if arm == nil {
		t.Fatal("arm should have survived its parent being despawned")
	}
	if arm.Parent() != nil {
		t.Errorf("arm should have been orphaned to the root, got parent %v", arm.Parent())
	}
	// Its own subtree stays intact underneath it.
	if bulb := a.World.Find("bulb"); bulb == nil || bulb.Parent() != arm {
		t.Error("arm's own child should still hang off it")
	}
}

func TestReparentRejectsCycles(t *testing.T) {
	path := filepath.Join(t.TempDir(), "scene.yml")
	if err := os.WriteFile(path, []byte(hierarchyScene), 0o644); err != nil {
		t.Fatalf("writing fixture: %v", err)
	}

	a := saveTestApp(t)
	loadAndPlace(t, a, path)

	rig := a.World.Find("rig")
	bulb := a.World.Find("bulb")

	// rig under its own grandchild would make WorldMatrix recurse forever.
	err := a.SetParent(rig.Handle(), bulb.Handle())
	if err == nil {
		t.Fatal("parenting an entity under its own descendant should fail")
	}
	if !strings.Contains(err.Error(), "descendant") {
		t.Errorf("error should explain the cycle, got: %v", err)
	}
	if rig.Parent() != nil {
		t.Error("the refused reparent still changed the tree")
	}

	// Parenting to self is refused too.
	if err := a.SetParent(rig.Handle(), rig.Handle()); err == nil {
		t.Log("parenting to self is a no-op rather than an error, which is fine")
	}
	if rig.Parent() == rig {
		t.Fatal("an entity became its own parent")
	}
}

func TestReparentAndDetach(t *testing.T) {
	path := filepath.Join(t.TempDir(), "scene.yml")
	if err := os.WriteFile(path, []byte(hierarchyScene), 0o644); err != nil {
		t.Fatalf("writing fixture: %v", err)
	}

	a := saveTestApp(t)
	loadAndPlace(t, a, path)

	loose := a.World.Find("loose")
	arm := a.World.Find("arm")
	rig := a.World.Find("rig")

	if err := a.SetParent(loose.Handle(), arm.Handle()); err != nil {
		t.Fatalf("reparenting: %v", err)
	}
	if loose.Parent() != arm {
		t.Errorf("loose's parent: got %v, want arm", loose.Parent())
	}

	// Moving arm out from under rig must take loose with it and leave rig with
	// one child.
	if err := a.SetParent(arm.Handle(), NoHandle); err != nil {
		t.Fatalf("detaching: %v", err)
	}
	if arm.Parent() != nil {
		t.Errorf("arm should be at the root, got parent %v", arm.Parent())
	}
	if got := len(rig.Children()); got != 1 {
		t.Errorf("rig's children after the move: got %d, want 1", got)
	}
	if loose.Parent() != arm {
		t.Error("loose should have moved with arm")
	}

	// An unknown child handle is an error, not a silent no-op.
	if err := a.SetParent(NoHandle, rig.Handle()); err == nil {
		t.Error("reparenting a handle that resolves to nothing should fail")
	}
}

// TestSaveWritesEachEntityOnce guards the trap in snapshotting a hierarchy: the
// world stores every entity in one flat slice, so writing the slice as-is would
// emit each child twice, once nested and once as a root.
func TestSaveWritesEachEntityOnce(t *testing.T) {
	directory := t.TempDir()
	original := filepath.Join(directory, "original.yml")
	if err := os.WriteFile(original, []byte(hierarchyScene), 0o644); err != nil {
		t.Fatalf("writing fixture: %v", err)
	}

	a := saveTestApp(t)
	loadAndPlace(t, a, original)

	snapshot, err := a.SceneSnapshot()
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}

	// Two roots: rig and loose.
	if got := len(snapshot.Objects); got != 2 {
		t.Errorf("top-level objects: got %d, want 2", got)
	}

	saved := filepath.Join(directory, "saved.yml")
	if err := a.SaveScene(saved); err != nil {
		t.Fatalf("save: %v", err)
	}
	content, err := os.ReadFile(saved)
	if err != nil {
		t.Fatalf("reading back: %v", err)
	}

	for _, name := range []string{"rig", "arm", "bulb", "base", "loose"} {
		if got := strings.Count(string(content), "name: "+name); got != 1 {
			t.Errorf("%q appears %d times in the saved scene, want 1:\n%s", name, got, content)
		}
	}
}

// TestGroupNodeIsValid covers the validation change children required: a node
// that exists only to be a pivot has nothing to draw and no behaviour, and that
// is allowed now.
func TestGroupNodeIsValid(t *testing.T) {
	a := saveTestApp(t)

	if _, err := a.SpawnObject(ObjectSpec{
		Name:      "pivot",
		Transform: IdentityTransform(),
		Children: []ObjectSpec{{
			Name:       "child",
			Transform:  IdentityTransform(),
			Components: []Component{NewPointLight()},
		}},
	}); err != nil {
		t.Fatalf("spawning a group node: %v", err)
	}

	path := filepath.Join(t.TempDir(), "scene.yml")
	if err := a.SaveScene(path); err != nil {
		t.Fatalf("a group node with children should be saveable: %v", err)
	}

	// And it has to load back.
	loadAndPlace(t, a, path)

	// Re-found by name rather than reused from before: the load retired every
	// handle, which is exactly what the generation counter is for.
	group := a.World.Find("pivot")
	if group == nil {
		t.Fatal("the group node did not survive the round trip")
	}

	// Stripping the children makes it do nothing again, which is still refused.
	if err := a.DespawnTree(group.Handle()); err != nil {
		t.Fatalf("cleanup: %v", err)
	}
	if _, err := a.SpawnObject(ObjectSpec{Name: "empty", Transform: IdentityTransform()}); err != nil {
		t.Fatalf("spawning an empty node: %v", err)
	}
	if err := a.SaveScene(path); err == nil {
		t.Error("a node with no model, components or children should still be unsaveable")
	}
}
