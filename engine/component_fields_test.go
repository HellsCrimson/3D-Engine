package engine

import (
	"strings"
	"testing"

	"github.com/go-gl/mathgl/mgl32"
)

// gadget covers every property shape the inspector has to present, plus the two
// it must refuse to.
type gadget struct {
	Speed    float32    `yaml:"speed"`
	Count    int        `yaml:"count"`
	Enabled  bool       `yaml:"enabled"`
	Tint     mgl32.Vec3 `yaml:"tint"`
	Rotation mgl32.Vec4 `yaml:"rotation"`
	Label    string     `yaml:"label"`

	// No tag: yaml lowercases the field name, and so must the inspector.
	Untagged float32

	// Excluded exactly as the saver excludes it.
	Skipped float32 `yaml:"-"`

	// A shape with no widget. Reported as unsupported rather than hidden.
	Table map[string]int `yaml:"table"`

	// Unexported: not part of what a scene file describes.
	private float32
}

func (g *gadget) Update(ctx *Context) {}

func fieldsByName(fields []ComponentField) map[string]ComponentField {
	byName := make(map[string]ComponentField, len(fields))
	for _, field := range fields {
		byName[field.Name] = field
	}
	return byName
}

func gadgetApp(t *testing.T) (*App, *gadget, Handle) {
	t.Helper()

	a := saveTestApp(t)
	a.Components.MustRegister("Gadget", func() Component { return &gadget{} })

	subject := &gadget{
		Speed:    12.5,
		Count:    7,
		Enabled:  true,
		Tint:     mgl32.Vec3{0.1, 0.2, 0.3},
		Rotation: mgl32.Vec4{0, 1, 0, 45},
		Label:    "hello",
		Untagged: 3,
		Skipped:  99,
		private:  1,
	}

	entity := NewEntity("subject")
	entity.AddComponent(subject)
	a.World.Spawn(entity)

	return a, subject, entity.Handle()
}

func TestComponentsOfReadsEveryShape(t *testing.T) {
	a, _, handle := gadgetApp(t)

	infos, ok := a.ComponentsOf(handle)
	if !ok {
		t.Fatal("entity not found")
	}
	if len(infos) != 1 {
		t.Fatalf("component count: got %d, want 1", len(infos))
	}
	if infos[0].Type != "Gadget" {
		t.Errorf("type name: got %q, want Gadget", infos[0].Type)
	}

	byName := fieldsByName(infos[0].Fields)

	if got := byName["speed"]; got.Kind != FieldFloat || got.Float != 12.5 {
		t.Errorf("speed: got %+v", got)
	}
	if got := byName["count"]; got.Kind != FieldInt || got.Int != 7 {
		t.Errorf("count: got %+v", got)
	}
	if got := byName["enabled"]; got.Kind != FieldBool || !got.Bool {
		t.Errorf("enabled: got %+v", got)
	}
	if got := byName["tint"]; got.Kind != FieldVec3 || got.Vec3 != (mgl32.Vec3{0.1, 0.2, 0.3}) {
		t.Errorf("tint: got %+v", got)
	}
	if got := byName["rotation"]; got.Kind != FieldVec4 || got.Vec4 != (mgl32.Vec4{0, 1, 0, 45}) {
		t.Errorf("rotation: got %+v", got)
	}
	if got := byName["label"]; got.Kind != FieldString || got.String != "hello" {
		t.Errorf("label: got %+v", got)
	}

	// An untagged field takes yaml's own default name.
	if got, ok := byName["untagged"]; !ok || got.Float != 3 {
		t.Errorf("untagged field: got %+v (present=%v)", got, ok)
	}

	// A shape with no widget is present but marked, not silently dropped.
	if got, ok := byName["table"]; !ok || got.Kind != FieldUnsupported {
		t.Errorf("table should be reported as unsupported, got %+v (present=%v)", got, ok)
	}
}

// TestComponentsOfSkipsWhatTheSaverSkips keeps the inspector and the save format
// describing the same set of properties. Editing something that could not be
// saved would silently lose the change on the next load.
func TestComponentsOfSkipsWhatTheSaverSkips(t *testing.T) {
	a, _, handle := gadgetApp(t)

	infos, _ := a.ComponentsOf(handle)
	byName := fieldsByName(infos[0].Fields)

	if _, present := byName["-"]; present {
		t.Error(`a field tagged yaml:"-" should not be editable`)
	}
	for _, field := range infos[0].Fields {
		if field.GoName == "Skipped" {
			t.Error("Skipped is excluded from saving, so it must be excluded here")
		}
		if field.GoName == "private" {
			t.Error("unexported fields must not be editable")
		}
	}
}

func TestSetComponentFieldWritesEveryShape(t *testing.T) {
	a, subject, handle := gadgetApp(t)

	edits := []ComponentField{
		{Name: "speed", Kind: FieldFloat, Float: 99.5},
		{Name: "count", Kind: FieldInt, Int: 42},
		{Name: "enabled", Kind: FieldBool, Bool: false},
		{Name: "tint", Kind: FieldVec3, Vec3: mgl32.Vec3{1, 0, 0}},
		{Name: "rotation", Kind: FieldVec4, Vec4: mgl32.Vec4{1, 0, 0, 90}},
		{Name: "label", Kind: FieldString, String: "changed"},
	}

	for _, edit := range edits {
		if err := a.SetComponentField(handle, 0, "Gadget", edit); err != nil {
			t.Fatalf("setting %s: %v", edit.Name, err)
		}
	}

	if subject.Speed != 99.5 || subject.Count != 42 || subject.Enabled {
		t.Errorf("scalars not applied: %+v", subject)
	}
	if subject.Tint != (mgl32.Vec3{1, 0, 0}) || subject.Rotation != (mgl32.Vec4{1, 0, 0, 90}) {
		t.Errorf("vectors not applied: %+v", subject)
	}
	if subject.Label != "changed" {
		t.Errorf("string not applied: %q", subject.Label)
	}
}

// TestSetComponentFieldChecksTheType is the guard that makes editing from a
// snapshot safe: a front-end addresses a component by index, and if the entity's
// components changed in between, the write must be refused rather than land on
// whatever now occupies that slot.
func TestSetComponentFieldChecksTheType(t *testing.T) {
	a, subject, handle := gadgetApp(t)

	err := a.SetComponentField(handle, 0, "PointLight", ComponentField{
		Name: "speed", Kind: FieldFloat, Float: 1,
	})
	if err == nil {
		t.Fatal("editing through a stale type name should fail")
	}
	if !strings.Contains(err.Error(), "Gadget") {
		t.Errorf("the error should say what is actually there, got: %v", err)
	}
	if subject.Speed != 12.5 {
		t.Error("the refused edit was applied anyway")
	}
}

func TestSetComponentFieldRejectsBadTargets(t *testing.T) {
	a, _, handle := gadgetApp(t)

	if err := a.SetComponentField(handle, 5, "Gadget", ComponentField{Name: "speed", Kind: FieldFloat}); err == nil {
		t.Error("an out-of-range component index should fail")
	}
	if err := a.SetComponentField(handle, 0, "Gadget", ComponentField{Name: "nope", Kind: FieldFloat}); err == nil {
		t.Error("an unknown property should fail")
	}
	if err := a.SetComponentField(handle, 0, "Gadget", ComponentField{Name: "skipped", Kind: FieldFloat, Float: 1}); err == nil {
		t.Error("a property excluded from the scene format should not be writable")
	}
	if err := a.SetComponentField(NoHandle, 0, "Gadget", ComponentField{Name: "speed", Kind: FieldFloat}); err == nil {
		t.Error("a stale handle should fail")
	}
}

// TestEditedComponentSurvivesASave closes the loop the whole feature rests on:
// a property changed in the inspector is a property the scene file records.
func TestEditedComponentSurvivesASave(t *testing.T) {
	a := saveTestApp(t)

	entity := NewEntity("lamp")
	entity.AddComponent(NewPointLight())
	a.World.Spawn(entity)

	if err := a.SetComponentField(entity.Handle(), 0, "PointLight", ComponentField{
		Name: "diffuse", Kind: FieldVec3, Vec3: mgl32.Vec3{0.25, 0.5, 0.75},
	}); err != nil {
		t.Fatalf("editing: %v", err)
	}

	path := t.TempDir() + "/scene.yml"
	if err := a.SaveScene(path); err != nil {
		t.Fatalf("save: %v", err)
	}
	loadAndPlace(t, a, path)

	reloaded := a.World.Find("lamp")
	if reloaded == nil {
		t.Fatal("lamp did not survive the round trip")
	}
	light, ok := GetComponent[*PointLight](reloaded)
	if !ok {
		t.Fatal("lamp lost its light")
	}
	if light.Diffuse != (mgl32.Vec3{0.25, 0.5, 0.75}) {
		t.Errorf("the edited value did not survive the save: got %v", light.Diffuse)
	}
}

// TestComponentsOfNamesUnregisteredTypes keeps a component the registry cannot
// name visible in the UI, since that is exactly the component that would fail a
// save and the user needs to see it.
func TestComponentsOfNamesUnregisteredTypes(t *testing.T) {
	a := saveTestApp(t)

	entity := NewEntity("mystery")
	entity.AddComponent(&gadget{Speed: 1})
	a.World.Spawn(entity)

	infos, ok := a.ComponentsOf(entity.Handle())
	if !ok || len(infos) != 1 {
		t.Fatalf("expected one component, got %v (found=%v)", infos, ok)
	}
	if infos[0].Type != "" {
		t.Errorf("an unregistered component should have no scene name, got %q", infos[0].Type)
	}
	if !strings.Contains(infos[0].GoType, "gadget") {
		t.Errorf("the Go type should still identify it, got %q", infos[0].GoType)
	}
	// Its properties are still readable, so the panel is not simply blank.
	if len(infos[0].Fields) == 0 {
		t.Error("an unregistered component should still show its properties")
	}
}

// TestBuiltInLightsAreFullyEditable is the practical check: the components that
// actually ship must present every property, since lights are the main thing
// anyone will want to tune live.
func TestBuiltInLightsAreFullyEditable(t *testing.T) {
	a := saveTestApp(t)

	entity := NewEntity("lights")
	entity.AddComponent(NewDirectionalLight(), NewPointLight(), NewSpotLight())
	a.World.Spawn(entity)

	infos, ok := a.ComponentsOf(entity.Handle())
	if !ok || len(infos) != 3 {
		t.Fatalf("expected three components, got %d", len(infos))
	}

	for _, info := range infos {
		if len(info.Fields) == 0 {
			t.Errorf("%s exposed no properties", info.Type)
		}
		for _, field := range info.Fields {
			if field.Kind == FieldUnsupported {
				t.Errorf("%s.%s has no editable representation", info.Type, field.Name)
			}
		}
	}

	// Spot-check that the names match what a scene file writes.
	byName := fieldsByName(infos[2].Fields)
	for _, want := range []string{"direction", "cutOff", "outerCutOff", "enabled", "followCamera"} {
		if _, ok := byName[want]; !ok {
			t.Errorf("SpotLight is missing property %q", want)
		}
	}
}
