package engine

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"3d-engine/camera"
	"3d-engine/scene"

	"github.com/go-gl/mathgl/mgl32"
)

// The scene used here deliberately has no models. Importing one needs a GL
// context, but every other part of the round trip — names, transforms, bodies,
// component types and their props — does not, so the interesting half of the
// format is covered by a real load/save/load cycle. Model paths are verified by
// running the binary.
const roundTripScene = `version: 2
skybox: ./testObjects/skybox
camera:
  position: [1.5, 2.5, -3.5]
  yaw: -120.0
  pitch: -15.0
objects:
  - name: sun
    transform:
      position: [0.0, 10.0, 0.0]
      rotation: [1.0, 0.0, 0.0, 45.0]
      scale: [1.0, 1.0, 1.0]
    components:
      - type: DirectionalLight
        props:
          direction: [-0.2, -1.0, -0.3]
          ambient: [0.1, 0.2, 0.3]
          diffuse: [0.4, 0.5, 0.6]
          specular: [0.7, 0.8, 0.9]

  - name: lamp
    transform:
      position: [2.0, 3.0, 4.0]
      scale: [0.5, 0.5, 0.5]
    body:
      static: true
    components:
      - type: PointLight
        props:
          diffuse: [1.0, 0.0, 0.0]
          linear: 0.14
          quadratic: 0.07

  - name: torch
    body:
      static: false
    components:
      - type: SpotLight
        props:
          followCamera: false
          enabled: true
          cutOff: 20.0
          outerCutOff: 25.0
      - type: PointLight
`

// saveTestApp builds an App with enough wired up to load and save a scene, and
// nothing that touches GL. skybox stays nil, which setSkybox tolerates, and
// Assets is never reached because no object here has a model.
func saveTestApp(t *testing.T) *App {
	t.Helper()

	a := &App{
		World:      NewWorld(),
		Components: NewComponentRegistry(),
		Camera: &camera.Camera{
			CameraPos: mgl32.Vec3{0, 0, 3},
			Yaw:       -90,
			Pitch:     0,
		},
	}
	registerBuiltinComponents(a.Components)
	a.Scenes = NewSceneManager(a, nil, "")
	a.World.onDespawn = a.destroyComponents

	return a
}

// loadAndPlace mirrors what the frame loop does on a scene change: load the
// scene, then move the camera to the spawn it declares. Without the second step
// the camera would never round-trip.
func loadAndPlace(t *testing.T, a *App, path string) {
	t.Helper()

	if err := a.Scenes.LoadScene(path); err != nil {
		t.Fatalf("loading %s: %v", path, err)
	}
	a.resetDynamicState()
}

// TestSceneRoundTripIsLossless is the guarantee the editor's Save button rests
// on: load a scene, save it, load it back, and the world is the same one.
//
// The comparison is between the two saved files rather than between two
// in-memory structs, because that is the artefact the user keeps, and it catches
// anything the encoder drops as well as anything the snapshot does.
func TestSceneRoundTripIsLossless(t *testing.T) {
	directory := t.TempDir()
	original := filepath.Join(directory, "original.yml")
	if err := os.WriteFile(original, []byte(roundTripScene), 0o644); err != nil {
		t.Fatalf("writing fixture: %v", err)
	}

	a := saveTestApp(t)
	loadAndPlace(t, a, original)

	firstSave := filepath.Join(directory, "first.yml")
	if err := a.SaveScene(firstSave); err != nil {
		t.Fatalf("first save: %v", err)
	}

	// Everything the first save wrote has to survive being read back and
	// written again.
	loadAndPlace(t, a, firstSave)

	secondSave := filepath.Join(directory, "second.yml")
	if err := a.SaveScene(secondSave); err != nil {
		t.Fatalf("second save: %v", err)
	}

	first, err := os.ReadFile(firstSave)
	if err != nil {
		t.Fatalf("reading first save: %v", err)
	}
	second, err := os.ReadFile(secondSave)
	if err != nil {
		t.Fatalf("reading second save: %v", err)
	}

	if string(first) != string(second) {
		t.Fatalf("round trip lost information.\nfirst save:\n%s\nsecond save:\n%s", first, second)
	}
}

// TestSceneRoundTripPreservesWorld checks the world itself, not just that two
// saves agree — two identically wrong files would compare equal.
func TestSceneRoundTripPreservesWorld(t *testing.T) {
	directory := t.TempDir()
	original := filepath.Join(directory, "original.yml")
	if err := os.WriteFile(original, []byte(roundTripScene), 0o644); err != nil {
		t.Fatalf("writing fixture: %v", err)
	}

	a := saveTestApp(t)
	loadAndPlace(t, a, original)

	saved := filepath.Join(directory, "saved.yml")
	if err := a.SaveScene(saved); err != nil {
		t.Fatalf("save: %v", err)
	}
	loadAndPlace(t, a, saved)

	if got := a.World.Len(); got != 3 {
		t.Fatalf("entity count after round trip: got %d, want 3", got)
	}

	// The camera is part of the scene, and saving captures where it actually is.
	if a.Camera.CameraPos != (mgl32.Vec3{1.5, 2.5, -3.5}) {
		t.Errorf("camera position: got %v, want {1.5 2.5 -3.5}", a.Camera.CameraPos)
	}
	if a.Camera.Yaw != -120 || a.Camera.Pitch != -15 {
		t.Errorf("camera orientation: got yaw %v pitch %v, want -120/-15", a.Camera.Yaw, a.Camera.Pitch)
	}

	lamp := a.World.Find("lamp")
	if lamp == nil {
		t.Fatal("lamp did not survive the round trip")
	}
	if lamp.Transform().Position != (mgl32.Vec3{2, 3, 4}) {
		t.Errorf("lamp position: got %v, want {2 3 4}", lamp.Transform().Position)
	}
	// The fixture omits rotation, so the loader's default has to survive too.
	if lamp.Transform().Rotation != (mgl32.Vec4{0, 1, 0, 0}) {
		t.Errorf("lamp rotation: got %v, want the default {0 1 0 0}", lamp.Transform().Rotation)
	}
	if lamp.Transform().Scale != (mgl32.Vec3{0.5, 0.5, 0.5}) {
		t.Errorf("lamp scale: got %v, want {0.5 0.5 0.5}", lamp.Transform().Scale)
	}
	if lamp.Body == nil || !lamp.Body.Static {
		t.Errorf("lamp body: got %+v, want a static body", lamp.Body)
	}

	point, ok := GetComponent[*PointLight](lamp)
	if !ok {
		t.Fatal("lamp lost its PointLight")
	}
	if point.Diffuse != (mgl32.Vec3{1, 0, 0}) {
		t.Errorf("point light diffuse: got %v, want {1 0 0}", point.Diffuse)
	}
	if point.Linear != 0.14 || point.Quadratic != 0.07 {
		t.Errorf("point light attenuation: got linear %v quadratic %v, want 0.14/0.07",
			point.Linear, point.Quadratic)
	}
	// Defaults the fixture never mentioned must come back as defaults, not zero.
	if point.Constant != 1.0 {
		t.Errorf("point light constant: got %v, want the default 1.0", point.Constant)
	}

	// A second component on the same entity has to survive alongside the first.
	torch := a.World.Find("torch")
	if torch == nil {
		t.Fatal("torch did not survive the round trip")
	}
	if got := len(torch.Components()); got != 2 {
		t.Fatalf("torch component count: got %d, want 2", got)
	}
	spot, ok := GetComponent[*SpotLight](torch)
	if !ok {
		t.Fatal("torch lost its SpotLight")
	}
	if spot.FollowCamera || spot.CutOff != 20 || spot.OuterCutOff != 25 {
		t.Errorf("spot light: got %+v, want followCamera=false cutOff=20 outerCutOff=25", spot)
	}
	if torch.Body == nil || torch.Body.Static {
		t.Errorf("torch body: got %+v, want a dynamic body", torch.Body)
	}
}

// TestSaveRefusesUnregisteredComponent covers the case that would otherwise lose
// behaviour silently: a component the registry cannot name is left out of the
// file, and the scene loads clean but does less than it used to.
func TestSaveRefusesUnregisteredComponent(t *testing.T) {
	a := saveTestApp(t)

	entity := NewEntity("mystery")
	entity.AddComponent(&probe{Label: "unregistered"})
	a.World.Spawn(entity)

	_, err := a.SceneSnapshot()
	if err == nil {
		t.Fatal("saving an unregistered component should fail rather than drop it")
	}
	if !strings.Contains(err.Error(), "mystery") {
		t.Errorf("error should name the offending object, got: %v", err)
	}
}

// TestSaveRefusesUnloadableScene stops the editor writing a file that its own
// loader would reject.
func TestSaveRefusesUnloadableScene(t *testing.T) {
	a := saveTestApp(t)

	// No model and no components: nothing for the loader to build.
	a.World.Spawn(NewEntity("empty"))

	path := filepath.Join(t.TempDir(), "broken.yml")
	if err := a.SaveScene(path); err == nil {
		t.Fatal("saving an object with neither a model nor components should fail")
	}

	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("a refused save must not leave a file behind")
	}
}

// TestSaveDoesNotClobberOnFailure guards the atomic-write path: an existing
// scene file must survive a save that fails.
func TestSaveDoesNotClobberOnFailure(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, "scene.yml")
	if err := os.WriteFile(path, []byte(roundTripScene), 0o644); err != nil {
		t.Fatalf("writing fixture: %v", err)
	}

	a := saveTestApp(t)
	loadAndPlace(t, a, path)
	a.World.Spawn(NewEntity("empty"))

	if err := a.SaveScene(path); err == nil {
		t.Fatal("expected the save to fail")
	}

	survived, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading the scene back: %v", err)
	}
	if string(survived) != roundTripScene {
		t.Fatal("a failed save damaged the existing scene file")
	}

	// The temporary file must not be left lying next to it either.
	entries, err := os.ReadDir(directory)
	if err != nil {
		t.Fatalf("listing the directory: %v", err)
	}
	if len(entries) != 1 {
		t.Errorf("expected only the scene file to remain, got %d entries", len(entries))
	}
}

// TestNameOfInvertsRegistration is what makes saving components possible at all.
func TestNameOfInvertsRegistration(t *testing.T) {
	r := NewComponentRegistry()
	registerBuiltinComponents(r)

	built, err := r.New("PointLight")
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	name, ok := r.NameOf(built)
	if !ok || name != "PointLight" {
		t.Fatalf("NameOf: got %q (%v), want %q", name, ok, "PointLight")
	}

	if _, ok := r.NameOf(&probe{}); ok {
		t.Error("NameOf claimed to know an unregistered type")
	}
	if _, ok := r.NameOf(nil); ok {
		t.Error("NameOf claimed to know nil")
	}
}

// TestRegistryRejectsSecondNameForSameType keeps the reverse mapping
// unambiguous — otherwise which name got saved would depend on map order.
func TestRegistryRejectsSecondNameForSameType(t *testing.T) {
	r := NewComponentRegistry()

	if err := r.Register("Lamp", func() Component { return NewPointLight() }); err != nil {
		t.Fatalf("first registration failed: %v", err)
	}
	if err := r.Register("Bulb", func() Component { return NewPointLight() }); err == nil {
		t.Fatal("registering a second name for the same Go type should fail")
	}
}

// TestSaveOmitsEmptyProps keeps saved files readable: a component with nothing
// to configure should not grow a props block.
func TestSaveOmitsEmptyProps(t *testing.T) {
	a := saveTestApp(t)
	a.Components.MustRegister("Marker", func() Component { return &marker{} })

	entity := NewEntity("marked")
	entity.AddComponent(&marker{})
	a.World.Spawn(entity)

	snapshot, err := a.SceneSnapshot()
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}

	path := filepath.Join(t.TempDir(), "scene.yml")
	if err := scene.Save(path, snapshot); err != nil {
		t.Fatalf("save: %v", err)
	}

	written, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading back: %v", err)
	}
	if strings.Contains(string(written), "props") {
		t.Errorf("a component with no properties should not write a props block:\n%s", written)
	}
}

// marker is a component with no configurable state, like a tag.
type marker struct{}
