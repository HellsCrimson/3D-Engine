package camera

import (
	"3d-engine/utils"
	"math"

	"github.com/go-gl/glfw/v3.3/glfw"
	"github.com/go-gl/mathgl/mgl32"
)

var (
	worldUp = mgl32.Vec3{0.0, 1.0, 0.0}
)

type Camera struct {
	CameraPos   mgl32.Vec3
	CameraFront mgl32.Vec3
	CameraUp    mgl32.Vec3
	CameraRight mgl32.Vec3
	CameraSpeed float32
	CameraFov   float32
	Yaw         float32
	Pitch       float32
	LastX       float32
	LastY       float32

	firstMouse        bool
	renderDistanceMin float32
	renderDistanceMax float32
}

func NewCamera(config *utils.Config) *Camera {
	return &Camera{
		CameraPos:   mgl32.Vec3{0.0, 0.0, 3.0},
		CameraFront: mgl32.Vec3{0.0, 0.0, -1.0},
		CameraUp:    mgl32.Vec3{0.0, 1.0, 0.0},
		CameraRight: mgl32.Vec3{1.0, 0.0, 0.0},
		CameraSpeed: config.CameraSpeed,
		CameraFov:   config.Fov,
		Yaw:         -90.0,
		Pitch:       0.0,
		LastX:       400,
		LastY:       300,

		firstMouse:        true,
		renderDistanceMin: config.RenderDistanceMin,
		renderDistanceMax: config.RenderDistanceMax,
	}
}

func (c *Camera) ProcessMovement(window *glfw.Window, deltaTime float32) {
	var cameraSpeed float32
	if window.GetKey(glfw.KeyLeftShift) == glfw.Press {
		cameraSpeed = c.CameraSpeed * 2.0
	} else {
		cameraSpeed = c.CameraSpeed
	}

	curCameraSpeed := cameraSpeed * deltaTime

	if window.GetKey(glfw.KeyW) == glfw.Press {
		c.CameraPos = c.CameraPos.Add(c.CameraFront.Mul(curCameraSpeed))
	}
	if window.GetKey(glfw.KeyS) == glfw.Press {
		c.CameraPos = c.CameraPos.Sub(c.CameraFront.Mul(curCameraSpeed))
	}
	if window.GetKey(glfw.KeyA) == glfw.Press {
		c.CameraPos = c.CameraPos.Sub(c.CameraFront.Cross(c.CameraUp).Mul(curCameraSpeed))
	}
	if window.GetKey(glfw.KeyD) == glfw.Press {
		c.CameraPos = c.CameraPos.Add(c.CameraFront.Cross(c.CameraUp).Mul(curCameraSpeed))
	}
	if window.GetKey(glfw.KeySpace) == glfw.Press {
		c.CameraPos = c.CameraPos.Add(c.CameraUp.Mul(curCameraSpeed))
	}
	if window.GetKey(glfw.KeyLeftControl) == glfw.Press {
		c.CameraPos = c.CameraPos.Sub(c.CameraUp.Mul(curCameraSpeed))
	}
}

func (c *Camera) MouseCallback(window *glfw.Window, xpos, ypos float64) {
	if c.firstMouse {
		c.LastX = float32(xpos)
		c.LastY = float32(ypos)
		c.firstMouse = false
	}

	xOffset := float32(xpos) - c.LastX
	yOffset := c.LastY - float32(ypos)
	c.LastX = float32(xpos)
	c.LastY = float32(ypos)

	var sensitivity float32 = 0.1
	xOffset *= sensitivity
	yOffset *= sensitivity

	c.Yaw += xOffset
	c.Pitch += yOffset

	if c.Pitch > 89.0 {
		c.Pitch = 89.0
	}
	if c.Pitch < -89.0 {
		c.Pitch = -89.0
	}

	direction := mgl32.Vec3{
		float32(math.Cos(float64(mgl32.DegToRad(c.Yaw))) * math.Cos(float64(mgl32.DegToRad(c.Pitch)))),
		float32(math.Sin(float64(mgl32.DegToRad(c.Pitch)))),
		float32(math.Sin(float64(mgl32.DegToRad(c.Yaw))) * math.Cos(float64(mgl32.DegToRad(c.Pitch)))),
	}

	c.CameraFront = direction.Normalize()

	c.CameraRight = c.CameraFront.Cross(worldUp).Normalize()
	c.CameraUp = c.CameraRight.Cross(c.CameraFront).Normalize()
}

func (c *Camera) ScrollCallback(window *glfw.Window, xOffset, yOffset float64) {
	c.CameraFov -= float32(yOffset)
	if c.CameraFov < 1.0 {
		c.CameraFov = 1.0
	}
	if c.CameraFov > 89.0 {
		c.CameraFov = 89
	}
}

func (c *Camera) ComputeView() mgl32.Mat4 {
	return mgl32.LookAtV(
		c.CameraPos,
		c.CameraPos.Add(c.CameraFront),
		c.CameraUp,
	)
}

func (c *Camera) ComputeProjection(width, height int) mgl32.Mat4 {
	return mgl32.Perspective(mgl32.DegToRad(c.CameraFov), float32(width)/float32(height), c.renderDistanceMin, c.renderDistanceMax)
}
