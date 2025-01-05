package camera

import (
	"3d-engine/utils"

	"github.com/go-gl/gl/v4.6-core/gl"
	"github.com/go-gl/glfw/v3.3/glfw"
)

type KeyHandler struct {
	KeyFunc   [glfw.KeyLast]func() bool
	IsPressed [glfw.KeyLast]bool
	LastPress [glfw.KeyLast]float64
}

func NewKeyHandler() *KeyHandler {
	kh := KeyHandler{}
	for i := 0; i < int(glfw.KeyLast); i++ {
		kh.KeyFunc[i] = EmptyFunc
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

func EmptyFunc() bool {
	return true
}

func (kh *KeyHandler) RegisterKeys(window *glfw.Window, cam *Camera, deltaTime *float32) {
	kh.KeyFunc[glfw.KeyEscape] = func() bool {
		window.SetShouldClose(true)
		return true
	}

	kh.KeyFunc[glfw.KeyW] = func() bool {
		cam.processForward(kh.IsPressed[glfw.KeyLeftShift], deltaTime)
		return true
	}

	kh.KeyFunc[glfw.KeyA] = func() bool {
		cam.processLeft(kh.IsPressed[glfw.KeyLeftShift], deltaTime)
		return true
	}

	kh.KeyFunc[glfw.KeyS] = func() bool {
		cam.processBack(kh.IsPressed[glfw.KeyLeftShift], deltaTime)
		return true
	}

	kh.KeyFunc[glfw.KeyD] = func() bool {
		cam.processRight(kh.IsPressed[glfw.KeyLeftShift], deltaTime)
		return true
	}

	kh.KeyFunc[glfw.KeySpace] = func() bool {
		cam.processUp(kh.IsPressed[glfw.KeyLeftShift], deltaTime)
		return true
	}

	kh.KeyFunc[glfw.KeyLeftControl] = func() bool {
		cam.processDown(kh.IsPressed[glfw.KeyLeftShift], deltaTime)
		return true
	}

	// Wireframe
	kh.KeyFunc[glfw.KeyZ] = func() bool {
		if glfw.GetTime()-kh.LastPress[glfw.KeyZ] >= 1 {
			if utils.GetContext().Wireframe {
				gl.PolygonMode(gl.FRONT_AND_BACK, gl.FILL)
			} else {
				gl.PolygonMode(gl.FRONT_AND_BACK, gl.LINE)
			}
			utils.GetContext().Wireframe = !utils.GetContext().Wireframe
			return true
		}
		return false
	}

	kh.KeyFunc[glfw.KeyF] = func() bool {
		if glfw.GetTime()-kh.LastPress[glfw.KeyF] >= 1 {
			utils.GetContext().FlashLight = !utils.GetContext().FlashLight
			return true
		}
		return false
	}
}
