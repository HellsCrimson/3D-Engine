package engine

import (
	"testing"

	"github.com/go-gl/mathgl/mgl32"
)

// TestNoHandleEncodesToZero is what makes parent_id backward compatible: a root
// object and a client that never sets the field both mean zero, so an old client
// talking to the new server is indistinguishable from one asking for the root.
func TestNoHandleEncodesToZero(t *testing.T) {
	if got := NoHandle.Encode(); got != 0 {
		t.Errorf("NoHandle encodes to %d, want 0", got)
	}
	if got := DecodeHandle(0); got != NoHandle {
		t.Errorf("0 decodes to %v, want NoHandle", got)
	}
	if !DecodeHandle(0).IsZero() {
		t.Error("a zero parent_id must not resolve to a real entity")
	}
}

// TestToProtoObjectReportsParent covers the gap this closed: the wire used to
// have nowhere to put the tree, so GET_OBJECTS returned it flattened with no way
// to tell a child from a root.
func TestToProtoObjectReportsParent(t *testing.T) {
	a := saveTestApp(t)

	parent, err := a.SpawnObject(ObjectSpec{
		Name:       "parent",
		Transform:  IdentityTransform(),
		Components: []Component{NewPointLight()},
	})
	if err != nil {
		t.Fatalf("spawning parent: %v", err)
	}

	child, err := a.SpawnObject(ObjectSpec{
		Name:       "child",
		Transform:  IdentityTransform(),
		Components: []Component{NewPointLight()},
		Parent:     parent.Handle(),
	})
	if err != nil {
		t.Fatalf("spawning child: %v", err)
	}

	parentInfo, _ := a.ObjectInfo(parent.Handle())
	childInfo, _ := a.ObjectInfo(child.Handle())

	if got := toProtoObject(parentInfo).GetParentId(); got != 0 {
		t.Errorf("a root object should report parent_id 0, got %d", got)
	}

	wireChild := toProtoObject(childInfo)
	if got := wireChild.GetParentId(); got != parent.Handle().Encode() {
		t.Errorf("child parent_id: got %d, want %d", got, parent.Handle().Encode())
	}
	// And the id round-trips back to the same handle, so a client can act on it.
	if got := DecodeHandle(wireChild.GetParentId()); got != parent.Handle() {
		t.Errorf("parent_id decodes to %v, want %v", got, parent.Handle())
	}
}

// TestToProtoObjectReportsAxisAngle keeps the wire format unchanged by the move
// to quaternions.
func TestToProtoObjectReportsAxisAngle(t *testing.T) {
	a := saveTestApp(t)

	spawned, err := a.SpawnObject(ObjectSpec{
		Name: "turned",
		Transform: Transform{
			Position: mgl32.Vec3{1, 2, 3},
			Rotation: QuatFromAxisAngle(mgl32.Vec4{0, 1, 0, 90}),
			Scale:    mgl32.Vec3{1, 1, 1},
		},
		Components: []Component{NewPointLight()},
	})
	if err != nil {
		t.Fatalf("spawning: %v", err)
	}

	info, _ := a.ObjectInfo(spawned.Handle())
	rotation := toProtoObject(info).GetLocation().GetRotation()

	if !nearly(rotation.GetW(), 90, 1e-3) {
		t.Errorf("angle on the wire: got %v, want 90 degrees", rotation.GetW())
	}
	if !nearly(rotation.GetY(), 1, 1e-4) {
		t.Errorf("axis on the wire: got (%v %v %v), want Y",
			rotation.GetX(), rotation.GetY(), rotation.GetZ())
	}
}
