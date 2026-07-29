package engine

import (
	"testing"

	"3d-engine/input"

	"github.com/go-gl/glfw/v3.3/glfw"
)

// fakeOverlay stands in for the editor: it can be shown and hidden, and it
// reports capturing input on demand.
type fakeOverlay struct {
	visible  bool
	mouse    bool
	keyboard bool
	frames   int
	closed   bool
}

func (f *fakeOverlay) Frame(a *App)           { f.frames++ }
func (f *fakeOverlay) CapturesMouse() bool    { return f.mouse }
func (f *fakeOverlay) CapturesKeyboard() bool { return f.keyboard }
func (f *fakeOverlay) Close()                 { f.closed = true }
func (f *fakeOverlay) Visible() bool          { return f.visible }
func (f *fakeOverlay) SetVisible(v bool)      { f.visible = v }

func TestToggleEditorFlipsVisibility(t *testing.T) {
	a := &App{}
	overlay := &fakeOverlay{visible: true}
	a.SetOverlay(overlay)

	a.toggleEditor()
	if overlay.visible {
		t.Fatal("toggle did not hide the overlay")
	}

	a.toggleEditor()
	if !overlay.visible {
		t.Fatal("toggle did not show the overlay again")
	}
}

// TestToggleEditorIgnoresOverlaysThatCannotHide keeps the toggle from panicking
// on an Overlay that does not implement Visible/SetVisible.
func TestToggleEditorIgnoresOverlaysThatCannotHide(t *testing.T) {
	a := &App{}
	a.SetOverlay(minimalOverlay{})
	a.toggleEditor()

	a.SetOverlay(nil)
	a.toggleEditor()
}

type minimalOverlay struct{}

func (minimalOverlay) Frame(a *App)           {}
func (minimalOverlay) CapturesMouse() bool    { return false }
func (minimalOverlay) CapturesKeyboard() bool { return false }
func (minimalOverlay) Close()                 {}

// TestEditorToggleSurvivesKeyboardCapture is a regression test.
//
// Poll blanks every action while the overlay owns the keyboard, so typing in a
// text field cannot also drive the camera. That initially blanked the editor
// toggle too, which would have trapped the user in a focused field with no way
// to dismiss the panel. defaultBindings marks the toggle always-active.
func TestEditorToggleSurvivesKeyboardCapture(t *testing.T) {
	m := input.NewMap()
	defaultBindings(m)

	// A frame where the overlay captures the keyboard and both F1 and W are held.
	held := heldKeys{glfw.KeyF1: true, glfw.KeyW: true}
	m.Poll(held, true)

	if !m.IsDown(ActionToggleEditor) {
		t.Fatal("the editor toggle must survive keyboard capture, or a focused text field traps the user")
	}
	if m.IsDown(ActionMoveForward) {
		t.Fatal("movement must be suppressed while the overlay owns the keyboard")
	}
}

// heldKeys is a KeySource for a fixed set of held keys.
type heldKeys map[glfw.Key]bool

func (h heldKeys) IsKeyDown(key glfw.Key) bool { return h[key] }
