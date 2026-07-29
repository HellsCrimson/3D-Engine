package components

import (
	"math"
	"testing"

	"3d-engine/engine"

	"github.com/go-gl/mathgl/mgl32"
	"gopkg.in/yaml.v3"
)

const tolerance = 1e-4

// step runs one Update against a bare context. Components only touch the entity
// and the delta, so nothing here needs an App or a GL context.
func step(component engine.Updater, entity *engine.Entity, deltaTime float32) {
	component.Update(&engine.Context{Entity: entity, DeltaTime: deltaTime})
}

func closeEnough(got, want float32) bool {
	return math.Abs(float64(got-want)) < tolerance
}

func vecCloseEnough(got, want mgl32.Vec3) bool {
	return closeEnough(got.X(), want.X()) &&
		closeEnough(got.Y(), want.Y()) &&
		closeEnough(got.Z(), want.Z())
}

func TestSpinnerAccumulatesAngle(t *testing.T) {
	entity := engine.NewEntity("subject")
	spinner := NewSpinner()
	spinner.Speed = 90

	// Two half-second steps have to land in the same place as one full second,
	// which is what makes the spin rate frame-rate independent.
	step(spinner, entity, 0.5)
	step(spinner, entity, 0.5)

	if !closeEnough(spinner.Angle, 90) {
		t.Errorf("angle after one second at 90 deg/s: got %v, want 90", spinner.Angle)
	}

	rotation := entity.Rotation()
	if !closeEnough(rotation.W(), 90) {
		t.Errorf("transform angle: got %v, want 90", rotation.W())
	}
	if rotation.Vec3() != (mgl32.Vec3{0, 1, 0}) {
		t.Errorf("spinner should write its own axis, got %v", rotation.Vec3())
	}
}

// TestSpinnerWrapsAngle keeps a long-running spinner from accumulating an
// unbounded angle, which would drift into float error and write ever-growing
// numbers into saved scenes.
func TestSpinnerWrapsAngle(t *testing.T) {
	entity := engine.NewEntity("subject")
	spinner := NewSpinner()
	spinner.Speed = 90
	spinner.Angle = 350

	step(spinner, entity, 1.0)

	if !closeEnough(spinner.Angle, 80) {
		t.Errorf("350 + 90 degrees should wrap to 80, got %v", spinner.Angle)
	}
}

func TestSpinnerReverses(t *testing.T) {
	entity := engine.NewEntity("subject")
	spinner := NewSpinner()
	spinner.Speed = -90
	spinner.Angle = 10

	step(spinner, entity, 1.0)

	// 10 - 90 = -80, which has to come back as 280 rather than a negative angle.
	if !closeEnough(spinner.Angle, 280) {
		t.Errorf("a negative speed should wrap below zero to 280, got %v", spinner.Angle)
	}
}

// TestSpinnerStartSeedsFromTransform covers the handover between the scene file's
// transform block and the component: an authored rotation angle is where the spin
// begins.
func TestSpinnerStartSeedsFromTransform(t *testing.T) {
	entity := engine.NewEntity("subject")
	entity.SetRotation(mgl32.Vec4{0, 1, 0, 45})

	spinner := NewSpinner()
	spinner.Start(&engine.Context{Entity: entity})

	if !closeEnough(spinner.Angle, 45) {
		t.Errorf("Start should adopt the transform's angle, got %v", spinner.Angle)
	}

	// An angle set in props wins: otherwise `angle: 0` could not mean zero.
	explicit := NewSpinner()
	explicit.Angle = 200
	explicit.Start(&engine.Context{Entity: entity})

	if !closeEnough(explicit.Angle, 200) {
		t.Errorf("Start should not overwrite an angle from props, got %v", explicit.Angle)
	}
}

// TestSpinnerSurvivesZeroAxis guards the NaN path: mgl32 normalises a zero vector
// to NaN, and a NaN rotation makes the entity disappear rather than error.
func TestSpinnerSurvivesZeroAxis(t *testing.T) {
	entity := engine.NewEntity("subject")
	spinner := &Spinner{Axis: mgl32.Vec3{0, 0, 0}, Speed: 90}

	step(spinner, entity, 0.5)

	rotation := entity.Rotation()
	for i := 0; i < 4; i++ {
		if math.IsNaN(float64(rotation[i])) {
			t.Fatalf("zero axis produced a NaN rotation: %v", rotation)
		}
	}
	if rotation.Vec3() != (mgl32.Vec3{0, 1, 0}) {
		t.Errorf("zero axis should fall back to Y, got %v", rotation.Vec3())
	}
}

func TestOrbiterStaysOnItsCircle(t *testing.T) {
	entity := engine.NewEntity("subject")
	orbiter := &Orbiter{
		Center: mgl32.Vec3{1, 2, 3},
		Axis:   mgl32.Vec3{0, 1, 0},
		Radius: 5,
		Speed:  90,
	}

	// A whole revolution in eight steps: every one of them has to sit on the
	// circle, at the centre's height, exactly the radius away.
	for i := 0; i < 8; i++ {
		step(orbiter, entity, 0.5)

		offset := entity.Position().Sub(orbiter.Center)
		if !closeEnough(offset.Len(), 5) {
			t.Fatalf("step %d: distance from centre is %v, want 5", i, offset.Len())
		}
		// The offset must lie in the plane the axis is normal to.
		if !closeEnough(offset.Dot(mgl32.Vec3{0, 1, 0}), 0) {
			t.Fatalf("step %d: orbit left its plane, offset %v", i, offset)
		}
	}

	// Four seconds at 90 deg/s is one full turn, back to where it started.
	if !closeEnough(orbiter.Phase, 0) && !closeEnough(orbiter.Phase, 360) {
		t.Errorf("phase after a full revolution: got %v, want 0", orbiter.Phase)
	}
}

// TestOrbiterQuarterTurn pins the actual geometry, not just its invariants, so a
// sign flip in the basis cannot pass unnoticed.
func TestOrbiterQuarterTurn(t *testing.T) {
	entity := engine.NewEntity("subject")
	orbiter := &Orbiter{
		Axis:   mgl32.Vec3{0, 1, 0},
		Radius: 2,
		Speed:  90,
		Phase:  -90, // so the first step lands exactly on phase 0
	}

	step(orbiter, entity, 1.0)
	if !vecCloseEnough(entity.Position(), mgl32.Vec3{0, 0, 2}) {
		t.Errorf("phase 0 about Y: got %v, want {0 0 2}", entity.Position())
	}

	step(orbiter, entity, 1.0)
	if !vecCloseEnough(entity.Position(), mgl32.Vec3{2, 0, 0}) {
		t.Errorf("phase 90 about Y: got %v, want {2 0 0}", entity.Position())
	}
}

// TestOrbiterHonoursItsAxis checks a non-default plane, since an axis-agnostic
// implementation would pass every Y-axis test above.
func TestOrbiterHonoursItsAxis(t *testing.T) {
	entity := engine.NewEntity("subject")
	orbiter := &Orbiter{
		Axis:   mgl32.Vec3{1, 0, 0},
		Radius: 3,
		Speed:  45,
	}

	for i := 0; i < 8; i++ {
		step(orbiter, entity, 1.0)

		if !closeEnough(entity.Position().X(), 0) {
			t.Fatalf("step %d: an orbit about X must stay at x=0, got %v", i, entity.Position())
		}
		if !closeEnough(entity.Position().Len(), 3) {
			t.Fatalf("step %d: radius drifted to %v", i, entity.Position().Len())
		}
	}
}

func TestOrbiterSurvivesZeroAxis(t *testing.T) {
	entity := engine.NewEntity("subject")
	orbiter := &Orbiter{Axis: mgl32.Vec3{0, 0, 0}, Radius: 4, Speed: 90}

	step(orbiter, entity, 0.5)

	position := entity.Position()
	for i := 0; i < 3; i++ {
		if math.IsNaN(float64(position[i])) {
			t.Fatalf("zero axis produced a NaN position: %v", position)
		}
	}
	if !closeEnough(position.Len(), 4) {
		t.Errorf("zero axis should fall back to a Y orbit of radius 4, got %v", position)
	}
}

// TestRegisterMakesComponentsAvailable is the end-to-end check for the pipeline
// this package exists to demonstrate: the name a scene file uses builds the right
// Go type, props decode onto it, and the engine can name it again for saving.
func TestRegisterMakesComponentsAvailable(t *testing.T) {
	registry := engine.NewComponentRegistry()

	if err := Register(registry); err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	for _, name := range []string{"Spinner", "Orbiter"} {
		built, err := registry.New(name)
		if err != nil {
			t.Fatalf("%s is not registered: %v", name, err)
		}
		// The reverse lookup is what lets a scene save these components.
		if got, ok := registry.NameOf(built); !ok || got != name {
			t.Errorf("NameOf(%s): got %q (%v)", name, got, ok)
		}
	}

	// Registering twice must fail rather than redefine what scene files mean.
	if err := Register(registry); err == nil {
		t.Error("registering the package twice should fail")
	}
}

// TestPropsDecodeOverDefaults is the rule component authors have to know: the
// factory runs first and props are decoded over it, so an omitted property keeps
// the constructor's value instead of becoming zero.
func TestPropsDecodeOverDefaults(t *testing.T) {
	registry := engine.NewComponentRegistry()
	if err := Register(registry); err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	var props yaml.Node
	if err := yaml.Unmarshal([]byte("radius: 12\nphase: 90\n"), &props); err != nil {
		t.Fatalf("fixture parse failed: %v", err)
	}

	built, err := registry.New("Orbiter")
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	if err := props.Decode(built); err != nil {
		t.Fatalf("props decode failed: %v", err)
	}

	orbiter := built.(*Orbiter)
	if orbiter.Radius != 12 || orbiter.Phase != 90 {
		t.Errorf("props not applied: %+v", orbiter)
	}
	if orbiter.Speed != NewOrbiter().Speed {
		t.Errorf("omitted speed should keep the default %v, got %v", NewOrbiter().Speed, orbiter.Speed)
	}
	if orbiter.Axis != (mgl32.Vec3{0, 1, 0}) {
		t.Errorf("omitted axis should keep the default Y, got %v", orbiter.Axis)
	}
}
