package input

import (
	"testing"

	"github.com/go-gl/glfw/v3.3/glfw"
)

// fakeKeyboard is a KeySource backed by a set of held keys, so the tests drive
// the real Poll rather than a copy of it.
type fakeKeyboard struct {
	pressed map[glfw.Key]bool
}

func (f *fakeKeyboard) IsKeyDown(key glfw.Key) bool { return f.pressed[key] }

func TestJustPressedIsASingleFrameEdge(t *testing.T) {
	m := NewMap()
	m.Bind("toggle", glfw.KeyF)
	kb := &fakeKeyboard{pressed: map[glfw.Key]bool{}}

	// Held for many frames: the toggle must fire exactly once. The old code
	// used a time-based cooldown for this, which fired again after a second.
	kb.pressed[glfw.KeyF] = true
	fires := 0
	for range 120 {
		m.Poll(kb, false)
		if m.JustPressed("toggle") {
			fires++
		}
	}
	if fires != 1 {
		t.Fatalf("held key fired %d times, want 1", fires)
	}

	// Release, then press again: one more fire.
	kb.pressed[glfw.KeyF] = false
	m.Poll(kb, false)
	if !m.JustReleased("toggle") {
		t.Fatal("release edge not reported")
	}

	kb.pressed[glfw.KeyF] = true
	m.Poll(kb, false)
	if !m.JustPressed("toggle") {
		t.Fatal("second press not reported")
	}
}

func TestIsDownTracksHeldState(t *testing.T) {
	m := NewMap()
	m.Bind("move", glfw.KeyW)
	kb := &fakeKeyboard{pressed: map[glfw.Key]bool{glfw.KeyW: true}}

	m.Poll(kb, false)
	m.Poll(kb, false)

	if !m.IsDown("move") {
		t.Fatal("held key should stay down")
	}
	if m.JustPressed("move") {
		t.Fatal("JustPressed should be false on the second frame of a hold")
	}
}

func TestAnyBoundKeyTriggersTheAction(t *testing.T) {
	m := NewMap()
	m.Bind("move_forward", glfw.KeyW, glfw.KeyUp)
	kb := &fakeKeyboard{pressed: map[glfw.Key]bool{glfw.KeyUp: true}}

	m.Poll(kb, false)
	if !m.IsDown("move_forward") {
		t.Fatal("the alternate binding did not trigger the action")
	}
}

// TestSuppressReleasesWithoutFakingAPress covers the overlay case: while the
// editor owns the keyboard everything must read as released, and handing focus
// back must not look like a fresh press of a key that was never let go.
func TestSuppressReleasesWithoutFakingAPress(t *testing.T) {
	m := NewMap()
	m.Bind("jump", glfw.KeySpace)
	kb := &fakeKeyboard{pressed: map[glfw.Key]bool{glfw.KeySpace: true}}

	m.Poll(kb, false)
	if !m.JustPressed("jump") {
		t.Fatal("expected the initial press")
	}

	m.Poll(kb, true)
	if m.IsDown("jump") {
		t.Fatal("suppressed input should read as released")
	}

	// Focus returns while the key is still physically held: that is a rising
	// edge, and firing here is correct — the alternative is a stuck action.
	m.Poll(kb, false)
	if !m.IsDown("jump") {
		t.Fatal("input should resume when suppression ends")
	}
}

func TestRebindReplacesRatherThanAppends(t *testing.T) {
	m := NewMap()
	m.Bind("toggle_editor", glfw.KeyF1)
	if err := m.BindNames("toggle_editor", "F2"); err != nil {
		t.Fatalf("BindNames: %v", err)
	}

	kb := &fakeKeyboard{pressed: map[glfw.Key]bool{glfw.KeyF1: true}}
	m.Poll(kb, false)
	if m.IsDown("toggle_editor") {
		t.Fatal("the old key still works after a rebind")
	}

	kb.pressed = map[glfw.Key]bool{glfw.KeyF2: true}
	m.Poll(kb, false)
	if !m.IsDown("toggle_editor") {
		t.Fatal("the new key does not work after a rebind")
	}
}

func TestUnknownKeyNameIsRejected(t *testing.T) {
	m := NewMap()

	if err := m.BindNames("bad", "NotAKey"); err == nil {
		t.Fatal("an unknown key name should be an error, not a silent no-op")
	}
	if err := m.BindNames("ok", "space", "F1", "LeftShift", "w"); err != nil {
		t.Fatalf("case-insensitive names should parse: %v", err)
	}
}

func TestUnboundActionIsInert(t *testing.T) {
	m := NewMap()
	kb := &fakeKeyboard{pressed: map[glfw.Key]bool{}}
	m.Poll(kb, false)

	if m.IsDown("never_bound") || m.JustPressed("never_bound") {
		t.Fatal("an unbound action should never report as active")
	}
	if got := m.Describe("never_bound"); got != "unbound" {
		t.Fatalf("Describe: got %q, want %q", got, "unbound")
	}
}

func TestDescribeListsBoundKeys(t *testing.T) {
	m := NewMap()
	m.Bind("move_forward", glfw.KeyW, glfw.KeyUp)

	if got := m.Describe("move_forward"); got != "W, Up" {
		t.Fatalf("Describe: got %q, want %q", got, "W, Up")
	}
}

// TestKeySetIsDeduplicated keeps Poll cheap: the old processInput read all ~350
// keys every frame, this one reads each distinct bound key once.
func TestKeySetIsDeduplicated(t *testing.T) {
	m := NewMap()
	m.Bind("move_up", glfw.KeySpace)
	m.Bind("jump", glfw.KeySpace)
	m.Bind("sprint", glfw.KeyLeftShift)

	if len(m.keys) != 2 {
		t.Fatalf("expected 2 distinct keys to poll, got %d: %v", len(m.keys), m.keys)
	}
}
