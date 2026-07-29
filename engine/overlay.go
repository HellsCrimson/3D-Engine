package engine

// Overlay is a 2D layer drawn on top of the finished 3D frame — an editor UI,
// a HUD, a debug console.
//
// It is an interface, and the engine core deliberately knows nothing about the
// UI toolkit behind it. That keeps ImGui's cgo dependency out of the engine and
// lets a game ship without one.
//
// Frame runs on the frame-loop goroutine after the scene has been rendered and
// before the buffers are swapped, so the overlay may make GL calls.
type Overlay interface {
	// Frame draws the overlay for this frame.
	Frame(a *App)

	// CapturesMouse reports whether the overlay is using the pointer this
	// frame, in which case the camera must not.
	CapturesMouse() bool

	// CapturesKeyboard reports whether the overlay is using the keyboard this
	// frame, in which case game key bindings must not fire.
	CapturesKeyboard() bool

	// Close releases the overlay's resources. The engine calls it on shutdown.
	Close()
}

// SetOverlay installs the overlay drawn on top of the scene. Pass nil to
// remove it. Call it after New, since an overlay usually needs a live GL
// context to initialise.
func (a *App) SetOverlay(overlay Overlay) {
	a.overlay = overlay
}

// overlayCapturesMouse is the guard the camera callbacks consult.
func (a *App) overlayCapturesMouse() bool {
	return a.overlay != nil && a.overlay.CapturesMouse()
}

// overlayCapturesKeyboard is the guard processInput consults.
func (a *App) overlayCapturesKeyboard() bool {
	return a.overlay != nil && a.overlay.CapturesKeyboard()
}
