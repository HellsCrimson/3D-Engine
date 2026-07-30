// Package editor draws the in-process editor UI on top of the running engine.
//
// It replaces the separate 3DEngineGUI process that drove the engine over gRPC:
// the panels here call the World API directly, so there is one binary to launch
// and one place to set a breakpoint. The gRPC server is still there for
// external tooling.
package editor

import (
	"fmt"

	"3d-engine/engine"

	"github.com/AllenDang/cimgui-go/imgui"
	"github.com/go-gl/mathgl/mgl32"
)

// Editor implements engine.Overlay.
type Editor struct {
	app *engine.App

	// selected is the entity the inspector is editing. It is a handle rather
	// than a pointer or an index, so a scene switch invalidates it cleanly
	// instead of silently inspecting whatever took its place.
	selected engine.Handle

	autoApply bool

	// draft holds the values the drag widgets write to. They are applied to the
	// entity, not read from it, while a drag is in progress — otherwise the
	// widget would fight the physics step for the same field.
	draft engine.Transform

	// draftRotation is the rotation the widgets actually edit, in the axis-angle
	// form a person can reason about. The transform stores a quaternion, which
	// has no component anyone would want to drag directly, so the inspector
	// converts on read and on apply.
	draftRotation mgl32.Vec4

	// draftColor is what the colour picker writes to. ImGui's ColorEdit3 wants a
	// [3]float32, not an mgl32.Vec3.
	draftColor [3]float32

	editing bool

	sceneModes    []string
	selectedScene int

	// savePath is where the Save button writes. It follows the loaded scene
	// until the user types a path of their own, which savePathEdited records —
	// otherwise a scene switch would silently retarget a deliberate Save As.
	savePath       string
	savePathEdited bool
	saveStatus     string

	// The create-entity form. Kept on the Editor rather than rebuilt per frame
	// because ImGui widgets write straight into these.
	spawnName      string
	spawnModel     string
	spawnComponent int32
	spawnWithBody  bool
	spawnStatic    bool
	spawnAsChild   bool
	spawnStatus    string

	// reparentTarget is a handle, not an index into the entity list. The list is
	// the world's dense slice, which reshuffles on every despawn, so an index
	// would silently come to mean a different entity.
	reparentTarget engine.Handle

	// status shows the last failed operation, e.g. editing an entity that was
	// despawned between frames.
	status string

	visible bool
}

// New attaches Dear ImGui to the app's existing window and GL context. Call it
// after engine.New, which is what creates them.
func New(app *engine.App) (*Editor, error) {
	imgui.CreateContext()

	io := imgui.CurrentIO()
	io.SetConfigFlags(io.ConfigFlags() | imgui.ConfigFlagsNavEnableKeyboard)

	if err := initBackends(app.Window.Handle(), "#version 460"); err != nil {
		return nil, fmt.Errorf("could not attach the editor UI: %w", err)
	}

	return &Editor{
		app:     app,
		visible: true,
	}, nil
}

// Frame draws the editor. The engine calls it after the 3D pass.
func (e *Editor) Frame(app *engine.App) {
	newFrame()
	imgui.NewFrame()

	if e.visible {
		e.draw()
	}

	render()
}

// CapturesMouse reports whether ImGui is using the pointer.
//
// While the cursor is captured for mouselook the editor never takes it, so the
// two input modes can't fight: press C to release the cursor and drive the UI,
// press it again to fly the camera.
func (e *Editor) CapturesMouse() bool {
	if !e.visible || e.app.State.CaptureCursor {
		return false
	}
	return imgui.CurrentIO().WantCaptureMouse()
}

// CapturesKeyboard reports whether ImGui is using the keyboard, which is true
// while a text field has focus.
func (e *Editor) CapturesKeyboard() bool {
	if !e.visible || e.app.State.CaptureCursor {
		return false
	}
	return imgui.CurrentIO().WantCaptureKeyboard()
}

// SetVisible shows or hides the editor without tearing it down.
func (e *Editor) SetVisible(visible bool) {
	e.visible = visible
}

// Visible reports whether the editor is being drawn.
func (e *Editor) Visible() bool {
	return e.visible
}

func (e *Editor) Close() {
	shutdownBackends()
	imgui.DestroyContext()
}
