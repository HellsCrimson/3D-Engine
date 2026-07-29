package engine

// State is the mutable runtime state that used to live in the process-wide
// utils.Context singleton. It now belongs to one App, so two Apps no longer
// fight over the same flags and game code reaches it through its App handle.
//
// It is read and written on the frame-loop goroutine only. Anything that needs
// to flip these from another goroutine has to go through the App.
type State struct {
	Wireframe         bool
	CaptureCursor     bool
	FlashLight        bool
	GravityEnabled    bool
	PlayerGravityMode bool
	CollisionDebug    bool
}
