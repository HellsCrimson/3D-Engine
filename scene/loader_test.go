package scene

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeScene(t *testing.T, body string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "scene.yml")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("writing fixture: %v", err)
	}
	return path
}

// TestMaterialDefaultsToWhite covers the same trap as the transform block: an
// omitted field must not collapse to zero. A material with no colour would
// otherwise mean black, and the object would disappear into shadow.
func TestMaterialDefaultsToWhite(t *testing.T) {
	path := writeScene(t, `version: 2
objects:
  - name: crate
    model: crate.obj
    material: {}
`)

	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	material := loaded.Objects[0].Material
	if material == nil {
		t.Fatal("material block was dropped")
	}
	if material.Color != [3]float32{1, 1, 1} {
		t.Errorf("colour: got %v, want white", material.Color)
	}
}

func TestMaterialColorIsRead(t *testing.T) {
	path := writeScene(t, `version: 2
objects:
  - name: crate
    model: crate.obj
    material:
      color: [0.2, 0.4, 0.6]
`)

	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	if got := loaded.Objects[0].Material.Color; got != [3]float32{0.2, 0.4, 0.6} {
		t.Errorf("colour: got %v, want {0.2 0.4 0.6}", got)
	}
}

// TestObjectWithoutMaterialHasNone keeps the block optional, which is what lets
// the saver leave it out for an object still using the default.
func TestObjectWithoutMaterialHasNone(t *testing.T) {
	path := writeScene(t, `version: 2
objects:
  - name: crate
    model: crate.obj
`)

	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	if loaded.Objects[0].Material != nil {
		t.Errorf("an omitted material should stay nil, got %+v", loaded.Objects[0].Material)
	}
}

// TestSaveRoundTripsMaterial goes through the file, since the encoder is what
// the engine's save path actually uses.
func TestSaveRoundTripsMaterial(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, "scene.yml")

	original := &Scene{
		Version: CurrentVersion,
		Objects: []Object{{
			Name:     "crate",
			Model:    "crate.obj",
			Material: &MaterialSpec{Color: [3]float32{0.9, 0.1, 0.35}},
		}},
	}

	if err := Save(path, original); err != nil {
		t.Fatalf("save: %v", err)
	}

	reloaded, err := Load(path)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}

	material := reloaded.Objects[0].Material
	if material == nil {
		t.Fatal("material did not survive the round trip")
	}
	if material.Color != original.Objects[0].Material.Color {
		t.Errorf("colour: got %v, want %v", material.Color, original.Objects[0].Material.Color)
	}

	// And it should have been written on one line, like every other vector.
	written, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading back: %v", err)
	}
	if want := "color: [0.9, 0.1, 0.35]"; !strings.Contains(string(written), want) {
		t.Errorf("expected %q in the saved scene:\n%s", want, written)
	}
}

// TestGroupNodeLoads pins the validation change children needed: an object with
// no model and no components is legitimate as long as it has children.
func TestGroupNodeLoads(t *testing.T) {
	path := writeScene(t, `version: 2
objects:
  - name: pivot
    children:
      - name: lamp
        components:
          - type: PointLight
`)

	if _, err := Load(path); err != nil {
		t.Fatalf("a grouping node with children should load: %v", err)
	}
}

func TestObjectThatDoesNothingIsRejected(t *testing.T) {
	path := writeScene(t, `version: 2
objects:
  - name: pointless
`)

	if _, err := Load(path); err == nil {
		t.Fatal("an object with no model, components or children should be rejected")
	}
}

// TestNestedObjectThatDoesNothingIsRejected checks the validation reaches all
// the way down, not just the top level.
func TestNestedObjectThatDoesNothingIsRejected(t *testing.T) {
	path := writeScene(t, `version: 2
objects:
  - name: pivot
    children:
      - name: alsoPointless
`)

	if _, err := Load(path); err == nil {
		t.Fatal("a nested object that does nothing should be rejected")
	}
}
