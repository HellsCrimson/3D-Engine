package engine

import (
	"github.com/go-gl/gl/v4.6-core/gl"
	"github.com/go-gl/glfw/v3.3/glfw"
)

// KeyHandler is a flat table of per-key callbacks. processInput polls every key
// each frame and calls PressKey, which records LastPress only when the callback
// returns true — that is how toggle keys debounce themselves.
//
// This lives in the engine rather than in camera because most of what it does
// (window input mode, polygon mode, engine flags) is not the camera's business.
type KeyHandler struct {
	KeyFunc   [glfw.KeyLast]func() bool
	IsPressed [glfw.KeyLast]bool
	LastPress [glfw.KeyLast]float64
}

func NewKeyHandler() *KeyHandler {
	kh := KeyHandler{}
	for i := 0; i < int(glfw.KeyLast); i++ {
		kh.KeyFunc[i] = emptyKeyFunc
		kh.IsPressed[i] = false
		kh.LastPress[i] = 0
	}
	return &kh
}

func (kh *KeyHandler) PressKey(key glfw.Key) {
	if kh.KeyFunc[key]() {
		kh.LastPress[key] = glfw.GetTime()
	}
}

// Bind replaces the callback for a key. Return true from fn to stamp LastPress,
// which is what the >= N second checks below debounce against.
func (kh *KeyHandler) Bind(key glfw.Key, fn func() bool) {
	kh.KeyFunc[key] = fn
}

// Elapsed reports the time since the last accepted press of key.
func (kh *KeyHandler) Elapsed(key glfw.Key) float64 {
	return glfw.GetTime() - kh.LastPress[key]
}

func emptyKeyFunc() bool {
	return true
}

func (a *App) registerDefaultKeys() {
	kh := a.Keys

	kh.KeyFunc[glfw.KeyEscape] = func() bool {
		a.Window.SetShouldClose(true)
		return true
	}

	kh.KeyFunc[glfw.KeyW] = func() bool {
		a.Camera.ProcessForward(kh.IsPressed[glfw.KeyLeftShift], a.State.PlayerGravityMode, a.deltaTime)
		return true
	}

	kh.KeyFunc[glfw.KeyA] = func() bool {
		a.Camera.ProcessLeft(kh.IsPressed[glfw.KeyLeftShift], a.deltaTime)
		return true
	}

	kh.KeyFunc[glfw.KeyS] = func() bool {
		a.Camera.ProcessBack(kh.IsPressed[glfw.KeyLeftShift], a.State.PlayerGravityMode, a.deltaTime)
		return true
	}

	kh.KeyFunc[glfw.KeyD] = func() bool {
		a.Camera.ProcessRight(kh.IsPressed[glfw.KeyLeftShift], a.deltaTime)
		return true
	}

	kh.KeyFunc[glfw.KeySpace] = func() bool {
		if a.State.PlayerGravityMode {
			return true
		}
		a.Camera.ProcessUp(kh.IsPressed[glfw.KeyLeftShift], a.deltaTime)
		return true
	}

	kh.KeyFunc[glfw.KeyLeftControl] = func() bool {
		if a.State.PlayerGravityMode {
			return true
		}
		a.Camera.ProcessDown(kh.IsPressed[glfw.KeyLeftShift], a.deltaTime)
		return true
	}

	// Wireframe
	kh.KeyFunc[glfw.KeyZ] = func() bool {
		if kh.Elapsed(glfw.KeyZ) >= 1 {
			if a.State.Wireframe {
				gl.PolygonMode(gl.FRONT_AND_BACK, gl.FILL)
			} else {
				gl.PolygonMode(gl.FRONT_AND_BACK, gl.LINE)
			}
			a.State.Wireframe = !a.State.Wireframe
			return true
		}
		return false
	}

	kh.KeyFunc[glfw.KeyF] = func() bool {
		if kh.Elapsed(glfw.KeyF) >= 1 {
			a.State.FlashLight = !a.State.FlashLight
			return true
		}
		return false
	}

	kh.KeyFunc[glfw.KeyC] = func() bool {
		if kh.Elapsed(glfw.KeyC) >= 1 {
			if a.State.CaptureCursor {
				a.Window.SetInputMode(glfw.CursorMode, glfw.CursorNormal)
			} else {
				a.Window.SetInputMode(glfw.CursorMode, glfw.CursorDisabled)
			}
			a.State.CaptureCursor = !a.State.CaptureCursor
			return true
		}
		return false
	}

	kh.KeyFunc[glfw.KeyG] = func() bool {
		if kh.Elapsed(glfw.KeyG) >= 0.3 {
			a.State.GravityEnabled = !a.State.GravityEnabled
			return true
		}
		return false
	}

	kh.KeyFunc[glfw.KeyP] = func() bool {
		if kh.Elapsed(glfw.KeyP) >= 0.3 {
			a.State.PlayerGravityMode = !a.State.PlayerGravityMode
			return true
		}
		return false
	}

	kh.KeyFunc[glfw.KeyB] = func() bool {
		if kh.Elapsed(glfw.KeyB) >= 0.3 {
			a.State.CollisionDebug = !a.State.CollisionDebug
			return true
		}
		return false
	}
}
