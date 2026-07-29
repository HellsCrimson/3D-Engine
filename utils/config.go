package utils

import (
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	RenderDistanceMin float32           `yaml:"renderDistanceMin"`
	RenderDistanceMax float32           `yaml:"renderDistanceMax"`
	Fov               float32           `yaml:"fov"`
	Width             int               `yaml:"width"`
	Height            int               `yaml:"height"`
	CameraSpeed       float32           `yaml:"cameraSpeed"`
	Vsync             bool              `yaml:"vsync"`
	DefaultSceneMode  string            `yaml:"defaultSceneMode"`
	SceneModes        map[string]string `yaml:"sceneModes"`

	Input   InputConfig   `yaml:"input"`
	Physics PhysicsConfig `yaml:"physics"`
	Player  PlayerConfig  `yaml:"player"`
	RPC     RPCConfig     `yaml:"rpc"`
}

// InputConfig rebinds actions. An action listed here replaces the engine
// default outright, so the old key stops working rather than both being live.
type InputConfig struct {
	Actions map[string][]string `yaml:"actions"`
}

// PhysicsConfig replaces the gravityStrength/gravityDirection package vars.
type PhysicsConfig struct {
	Gravity float32 `yaml:"gravity"`
	// GravityAxes are the directions the H key cycles through. The first is the
	// one a scene starts with.
	GravityAxes [][3]float32 `yaml:"gravityAxes"`
	// CollisionDebugDistance is how far from the camera debug boxes are drawn.
	CollisionDebugDistance float32 `yaml:"collisionDebugDistance"`
}

// PlayerConfig replaces the player capsule constants.
type PlayerConfig struct {
	HalfExtents  [3]float32 `yaml:"halfExtents"`
	CenterOffset [3]float32 `yaml:"centerOffset"`
	JumpSpeed    float32    `yaml:"jumpSpeed"`
}

// RPCConfig controls the editor server. A shipped game normally disables it
// rather than opening a port.
type RPCConfig struct {
	Address string `yaml:"address"`
	Disable bool   `yaml:"disable"`
}

// applyDefaults fills in anything the file left out with the values the engine
// used to hardcode, so a minimal config still behaves as before.
func (c *Config) applyDefaults() {
	if c.Physics.Gravity == 0 {
		c.Physics.Gravity = 9.81
	}
	if len(c.Physics.GravityAxes) == 0 {
		c.Physics.GravityAxes = [][3]float32{
			{0.0, -1.0, 0.0},
			{0.0, 0.0, -1.0},
		}
	}
	if c.Physics.CollisionDebugDistance == 0 {
		c.Physics.CollisionDebugDistance = 80.0
	}

	if c.Player.HalfExtents == ([3]float32{}) {
		c.Player.HalfExtents = [3]float32{0.35, 0.9, 0.35}
	}
	if c.Player.CenterOffset == ([3]float32{}) {
		c.Player.CenterOffset = [3]float32{0.0, -0.9, 0.0}
	}
	if c.Player.JumpSpeed == 0 {
		c.Player.JumpSpeed = 6.0
	}

	if c.RPC.Address == "" {
		c.RPC.Address = "localhost:8080"
	}
}

func LoadConfig(path string) (*Config, error) {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil, Logger().Errorf("config file does not exist: %s", path)
	}

	fileContent, err := os.ReadFile(path)
	if err != nil {
		return nil, Logger().Errorf("failed to read config file: %s", err)
	}

	config := &Config{}
	if err := yaml.Unmarshal(fileContent, config); err != nil {
		return nil, Logger().Errorf("failed to parse YAML config: %s", err)
	}
	config.applyDefaults()

	return config, nil
}

func (c *Config) GetVsync() int {
	if c.Vsync {
		return 1
	} else {
		return 0
	}
}
