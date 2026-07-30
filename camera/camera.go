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

// Step returns the distance to travel this frame.
func (c *Camera) Step(isRunning bool, deltaTime float32) float32 {
	speed := c.CameraSpeed
	if isRunning {
		speed *= 2.0
	}
	return speed * deltaTime
}

// forward returns the view direction, flattened onto the XZ plane when planar
// is set so that looking up or down does not lift a walking player.
func (c *Camera) forward(planar bool) (mgl32.Vec3, bool) {
	if !planar {
		return c.CameraFront, true
	}

	flat := mgl32.Vec3{c.CameraFront.X(), 0, c.CameraFront.Z()}
	if flat.LenSqr() < 1e-6 {
		return mgl32.Vec3{}, false
	}
	return flat.Normalize(), true
}

func (c *Camera) ProcessForward(isRunning, planar bool, deltaTime float32) {
	forward, ok := c.forward(planar)
	if !ok {
		return
	}
	c.CameraPos = c.CameraPos.Add(forward.Mul(c.Step(isRunning, deltaTime)))
}

func (c *Camera) ProcessBack(isRunning, planar bool, deltaTime float32) {
	forward, ok := c.forward(planar)
	if !ok {
		return
	}
	c.CameraPos = c.CameraPos.Sub(forward.Mul(c.Step(isRunning, deltaTime)))
}

func (c *Camera) ProcessLeft(isRunning bool, deltaTime float32) {
	c.CameraPos = c.CameraPos.Sub(c.CameraRight.Mul(c.Step(isRunning, deltaTime)))
}

func (c *Camera) ProcessRight(isRunning bool, deltaTime float32) {
	c.CameraPos = c.CameraPos.Add(c.CameraRight.Mul(c.Step(isRunning, deltaTime)))
}

func (c *Camera) ProcessUp(isRunning bool, deltaTime float32) {
	c.CameraPos = c.CameraPos.Add(c.CameraUp.Mul(c.Step(isRunning, deltaTime)))
}

func (c *Camera) ProcessDown(isRunning bool, deltaTime float32) {
	c.CameraPos = c.CameraPos.Sub(c.CameraUp.Mul(c.Step(isRunning, deltaTime)))
}

// ResetMouse makes the next MouseCallback establish a new origin instead of
// measuring a delta from the last one.
//
// It is what keeps releasing and re-capturing the cursor from snapping the view:
// while the cursor is free the pointer moves without the camera hearing about it,
// so LastX/LastY go stale, and the first position after re-capture would
// otherwise be read as one enormous mouse movement.
func (c *Camera) ResetMouse() {
	c.firstMouse = true
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

	c.updateVectors()
}

// updateVectors rebuilds the front/right/up basis from the current yaw and
// pitch.
func (c *Camera) updateVectors() {
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

// SetOrientation points the camera and rebuilds its basis vectors. Used when a
// scene defines a spawn orientation.
func (c *Camera) SetOrientation(yaw, pitch float32) {
	c.Yaw = yaw
	c.Pitch = pitch
	c.firstMouse = true
	c.updateVectors()
}
