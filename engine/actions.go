package engine

import (
	"3d-engine/input"
	"3d-engine/utils"

	"github.com/go-gl/gl/v4.6-core/gl"
	"github.com/go-gl/glfw/v3.3/glfw"
)

// The actions the engine reacts to. A game binds its own alongside these; the
// engine only knows the names, not the keys.
const (
	ActionMoveForward = input.Action("move_forward")
	ActionMoveBack    = input.Action("move_back")
	ActionMoveLeft    = input.Action("move_left")
	ActionMoveRight   = input.Action("move_right")
	ActionMoveUp      = input.Action("move_up")
	ActionMoveDown    = input.Action("move_down")
	ActionJump        = input.Action("jump")
	ActionSprint      = input.Action("sprint")

	ActionQuit               = input.Action("quit")
	ActionToggleCursor       = input.Action("toggle_cursor")
	ActionToggleWireframe    = input.Action("toggle_wireframe")
	ActionToggleFlashlight   = input.Action("toggle_flashlight")
	ActionToggleGravity      = input.Action("toggle_gravity")
	ActionCycleGravityAxis   = input.Action("cycle_gravity_axis")
	ActionTogglePlayerMode   = input.Action("toggle_player_mode")
	ActionToggleCollisionBox = input.Action("toggle_collision_debug")
	ActionToggleEditor       = input.Action("toggle_editor")
)

// defaultBindings reproduce the keys the engine used to hardcode, plus F1 for
// the editor overlay.
func defaultBindings(m *input.Map) {
	m.Bind(ActionMoveForward, glfw.KeyW)
	m.Bind(ActionMoveBack, glfw.KeyS)
	m.Bind(ActionMoveLeft, glfw.KeyA)
	m.Bind(ActionMoveRight, glfw.KeyD)
	m.Bind(ActionMoveUp, glfw.KeySpace)
	m.Bind(ActionMoveDown, glfw.KeyLeftControl)
	m.Bind(ActionJump, glfw.KeySpace)
	m.Bind(ActionSprint, glfw.KeyLeftShift)

	m.Bind(ActionQuit, glfw.KeyEscape)
	m.Bind(ActionToggleCursor, glfw.KeyC)
	m.Bind(ActionToggleWireframe, glfw.KeyZ)
	m.Bind(ActionToggleFlashlight, glfw.KeyF)
	m.Bind(ActionToggleGravity, glfw.KeyG)
	m.Bind(ActionCycleGravityAxis, glfw.KeyH)
	m.Bind(ActionTogglePlayerMode, glfw.KeyP)
	m.Bind(ActionToggleCollisionBox, glfw.KeyB)
	m.Bind(ActionToggleEditor, glfw.KeyF1)

	// Exempt from suppression: without this, focusing a text field in the
	// editor would make the key that closes the editor stop working.
	m.SetAlwaysActive(ActionToggleEditor)
}

// applyConfigBindings lets the config file rebind anything. An action named in
// config replaces its default outright rather than adding to it, so a rebind
// does not leave the old key working.
func applyConfigBindings(m *input.Map, bindings map[string][]string) error {
	for name, keys := range bindings {
		if err := m.BindNames(input.Action(name), keys...); err != nil {
			return err
		}
	}
	return nil
}

// handleActions runs the engine's own reactions to input. Continuous actions
// read IsDown; toggles read JustPressed, which is a rising edge and therefore
// self-debouncing — the old code hand-rolled a timing check per key.
func (a *App) handleActions() {
	in := a.Input

	// The editor toggle is checked before anything else so it still works while
	// the overlay has keyboard focus; otherwise a focused text field could trap
	// you in the UI.
	if in.JustPressed(ActionToggleEditor) {
		a.toggleEditor()
	}

	if a.overlayCapturesKeyboard() {
		return
	}

	if in.JustPressed(ActionQuit) {
		a.Quit()
	}

	a.handleMovement()

	if in.JustPressed(ActionToggleCursor) {
		a.setCursorCaptured(!a.State.CaptureCursor)
	}
	if in.JustPressed(ActionToggleWireframe) {
		a.setWireframe(!a.State.Wireframe)
	}
	if in.JustPressed(ActionToggleFlashlight) {
		a.State.FlashLight = !a.State.FlashLight
	}
	if in.JustPressed(ActionToggleGravity) {
		a.State.GravityEnabled = !a.State.GravityEnabled
	}
	if in.JustPressed(ActionCycleGravityAxis) {
		a.cycleGravityAxis()
	}
	if in.JustPressed(ActionTogglePlayerMode) {
		a.State.PlayerGravityMode = !a.State.PlayerGravityMode
	}
	if in.JustPressed(ActionToggleCollisionBox) {
		a.State.CollisionDebug = !a.State.CollisionDebug
	}
}

func (a *App) handleMovement() {
	in := a.Input
	sprint := in.IsDown(ActionSprint)
	// In player-gravity mode movement is flattened onto the ground plane so
	// looking up does not lift the player off it.
	planar := a.State.PlayerGravityMode

	if in.IsDown(ActionMoveForward) {
		a.Camera.ProcessForward(sprint, planar, a.deltaTime)
	}
	if in.IsDown(ActionMoveBack) {
		a.Camera.ProcessBack(sprint, planar, a.deltaTime)
	}
	if in.IsDown(ActionMoveLeft) {
		a.Camera.ProcessLeft(sprint, a.deltaTime)
	}
	if in.IsDown(ActionMoveRight) {
		a.Camera.ProcessRight(sprint, a.deltaTime)
	}

	// Space and LeftCtrl fly the camera only when the player is not on the
	// ground rig; in player mode Space is the jump, handled by the physics step.
	if a.State.PlayerGravityMode {
		return
	}
	if in.IsDown(ActionMoveUp) {
		a.Camera.ProcessUp(sprint, a.deltaTime)
	}
	if in.IsDown(ActionMoveDown) {
		a.Camera.ProcessDown(sprint, a.deltaTime)
	}
}

func (a *App) setCursorCaptured(captured bool) {
	a.State.CaptureCursor = captured
	if captured {
		// The pointer moved while it was free and the camera did not follow, so
		// its idea of where the mouse was is stale. Without this the first
		// position after re-capturing reads as one huge movement and the view
		// snaps somewhere else.
		a.Camera.ResetMouse()
		a.Window.SetInputMode(glfw.CursorMode, glfw.CursorDisabled)
		return
	}
	a.Window.SetInputMode(glfw.CursorMode, glfw.CursorNormal)
}

func (a *App) setWireframe(enabled bool) {
	a.State.Wireframe = enabled
	if enabled {
		gl.PolygonMode(gl.FRONT_AND_BACK, gl.LINE)
		return
	}
	gl.PolygonMode(gl.FRONT_AND_BACK, gl.FILL)
}

// toggleEditor shows or hides the overlay, if one is installed and it supports
// being hidden.
func (a *App) toggleEditor() {
	toggler, ok := a.overlay.(interface {
		Visible() bool
		SetVisible(bool)
	})
	if !ok {
		return
	}

	toggler.SetVisible(!toggler.Visible())
	utils.Logger().Infoln("Editor overlay visible:", toggler.Visible())
}
