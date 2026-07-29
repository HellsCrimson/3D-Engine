package engine

import (
	"testing"

	"github.com/go-gl/mathgl/mgl32"
)

func lightEntity(name string, position mgl32.Vec3, components ...Component) *Entity {
	entity := NewEntity(name)
	entity.SetPosition(position)
	entity.AddComponent(components...)
	return entity
}

func TestCollectGathersEachLightKind(t *testing.T) {
	lights := lightSet{}

	lights.collect(lightEntity("sun", mgl32.Vec3{}, NewDirectionalLight()))
	lights.collect(lightEntity("lamp", mgl32.Vec3{3, 4, 5}, NewPointLight()))
	lights.collect(lightEntity("torch", mgl32.Vec3{1, 1, 1}, NewSpotLight()))

	if lights.directional == nil {
		t.Fatal("directional light not collected")
	}
	if len(lights.points) != 1 {
		t.Fatalf("expected 1 point light, got %d", len(lights.points))
	}
	if lights.spot == nil {
		t.Fatal("spot light not collected")
	}

	// A point light's position comes from its entity, not from the component.
	if got := lights.points[0].position; got != (mgl32.Vec3{3, 4, 5}) {
		t.Fatalf("point light position: got %v, want {3 4 5}", got)
	}
}

// TestCollectRespectsShaderPointLightLimit guards against silently overflowing
// the fixed-size pointLights array in the fragment shader.
func TestCollectRespectsShaderPointLightLimit(t *testing.T) {
	lights := lightSet{}

	const extra = 3
	for i := 0; i < MaxPointLights+extra; i++ {
		lights.collect(lightEntity("lamp", mgl32.Vec3{float32(i), 0, 0}, NewPointLight()))
	}

	if len(lights.points) != MaxPointLights {
		t.Fatalf("collected %d point lights, shader accepts %d", len(lights.points), MaxPointLights)
	}
	if lights.droppedPoints != extra {
		t.Fatalf("dropped count: got %d, want %d", lights.droppedPoints, extra)
	}
}

// TestCollectKeepsFirstOfSingletonLights documents the tie-break: the shader
// has one dirLight and one spotLight slot, so the first found wins rather than
// the last silently overwriting.
func TestCollectKeepsFirstOfSingletonLights(t *testing.T) {
	first := NewDirectionalLight()
	first.Diffuse = mgl32.Vec3{1, 0, 0}
	second := NewDirectionalLight()
	second.Diffuse = mgl32.Vec3{0, 1, 0}

	firstSpot := NewSpotLight()
	firstSpot.CutOff = 5
	secondSpot := NewSpotLight()
	secondSpot.CutOff = 45

	lights := lightSet{}
	lights.collect(lightEntity("sun-a", mgl32.Vec3{}, first, firstSpot))
	lights.collect(lightEntity("sun-b", mgl32.Vec3{}, second, secondSpot))

	if lights.directional.Diffuse != (mgl32.Vec3{1, 0, 0}) {
		t.Fatalf("expected the first directional light to win, got %v", lights.directional.Diffuse)
	}
	if lights.spot.light.CutOff != 5 {
		t.Fatalf("expected the first spot light to win, got %v", lights.spot.light.CutOff)
	}
}

// TestCollectUsesWorldPositionForParentedLights checks that a light attached
// under a parent is placed in world space, not local space.
func TestCollectUsesWorldPositionForParentedLights(t *testing.T) {
	parent := NewEntity("rig")
	parent.SetPosition(mgl32.Vec3{10, 0, 0})

	lamp := lightEntity("lamp", mgl32.Vec3{0, 5, 0}, NewPointLight())
	lamp.SetParent(parent)

	lights := lightSet{}
	lights.collect(lamp)

	if got := lights.points[0].position; got != (mgl32.Vec3{10, 5, 0}) {
		t.Fatalf("parented light position: got %v, want {10 5 0}", got)
	}
}

// TestEntitiesWithoutLightsAreIgnored keeps collect cheap and side-effect free
// for the overwhelmingly common case.
func TestEntitiesWithoutLightsAreIgnored(t *testing.T) {
	lights := lightSet{}
	lights.collect(lightEntity("crate", mgl32.Vec3{}, &RigidBody{}))

	if lights.directional != nil || lights.spot != nil || len(lights.points) != 0 {
		t.Fatal("a non-light entity contributed lights")
	}
}
