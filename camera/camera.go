package camera

import (
	"math"

	"github.com/go-gl/glfw/v3.3/glfw"
	"github.com/go-gl/mathgl/mgl32"
)

type Camera struct {
	CameraPos   mgl32.Vec3
	CameraFront mgl32.Vec3
	CameraUp    mgl32.Vec3
	CameraSpeed float32
	CameraFov   float32
	Yaw         float32
	Pitch       float32
	LastX       float32
	LastY       float32
	firstMouse  bool
}

func NewCamera() *Camera {
	return &Camera{
		CameraPos:   mgl32.Vec3{0.0, 0.0, 3.0},
		CameraFront: mgl32.Vec3{0.0, 0.0, -1.0},
		CameraUp:    mgl32.Vec3{0.0, 1.0, 0.0},
		CameraSpeed: 2.5,
		CameraFov:   45.0,
		Yaw:         -90.0,
		Pitch:       0.0,
		LastX:       400,
		LastY:       300,
		firstMouse:  true,
	}
}

func (c *Camera) ProcessMovement(window *glfw.Window, deltaTime float32) {
	curCameraSpeed := c.CameraSpeed * deltaTime

	if window.GetKey(glfw.KeyW) == glfw.Press {
		c.CameraPos = c.CameraPos.Add(c.CameraFront.Mul(curCameraSpeed).Normalize())
	}
	if window.GetKey(glfw.KeyS) == glfw.Press {
		c.CameraPos = c.CameraPos.Sub(c.CameraFront.Mul(curCameraSpeed).Normalize())
	}
	if window.GetKey(glfw.KeyA) == glfw.Press {
		c.CameraPos = c.CameraPos.Sub(c.CameraFront.Cross(c.CameraUp).Mul(curCameraSpeed).Normalize())
	}
	if window.GetKey(glfw.KeyD) == glfw.Press {
		c.CameraPos = c.CameraPos.Add(c.CameraFront.Cross(c.CameraUp).Mul(curCameraSpeed).Normalize())
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
}

func (c *Camera) ScrollCallback(window *glfw.Window, xOffset, yOffset float64) {
	c.CameraFov -= float32(yOffset)
	if c.CameraFov < 1.0 {
		c.CameraFov = 1.0
	}
	if c.CameraFov > 45.0 {
		c.CameraFov = 45
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
	return mgl32.Perspective(mgl32.DegToRad(c.CameraFov), float32(width)/float32(height), 0.1, 100.0)
}
