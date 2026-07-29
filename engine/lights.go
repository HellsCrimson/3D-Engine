package engine

import "github.com/go-gl/mathgl/mgl32"

// MaxPointLights mirrors MAX_POINT_LIGHT in shaders/lighting.frag. Lights past
// this count are dropped with a warning rather than silently ignored.
const MaxPointLights = 4

// DirectionalLight is the sun: a direction and colours, no position. Attach one
// to an entity and the renderer picks it up.
//
// Direction is world-space and explicit rather than derived from the entity's
// rotation, because axis-angle to forward-vector is a surprising step to hide
// behind a light's placement.
type DirectionalLight struct {
	Direction mgl32.Vec3 `yaml:"direction"`
	Ambient   mgl32.Vec3 `yaml:"ambient"`
	Diffuse   mgl32.Vec3 `yaml:"diffuse"`
	Specular  mgl32.Vec3 `yaml:"specular"`
}

// NewDirectionalLight returns the light with the values that used to be
// hardcoded in computeLight, so a scene that just says `type:
// DirectionalLight` looks like the old built-in one.
func NewDirectionalLight() *DirectionalLight {
	return &DirectionalLight{
		Direction: mgl32.Vec3{-0.2, -1.0, -0.3},
		Ambient:   mgl32.Vec3{0.2, 0.2, 0.2},
		Diffuse:   mgl32.Vec3{0.5, 0.5, 0.5},
		Specular:  mgl32.Vec3{1.0, 1.0, 1.0},
	}
}

// PointLight radiates from its entity's world position.
type PointLight struct {
	Ambient  mgl32.Vec3 `yaml:"ambient"`
	Diffuse  mgl32.Vec3 `yaml:"diffuse"`
	Specular mgl32.Vec3 `yaml:"specular"`

	Constant  float32 `yaml:"constant"`
	Linear    float32 `yaml:"linear"`
	Quadratic float32 `yaml:"quadratic"`
}

// NewPointLight uses the attenuation from the LearnOpenGL tables that the
// commented-out point-light block in computeLight was going to use.
func NewPointLight() *PointLight {
	return &PointLight{
		Ambient:   mgl32.Vec3{0.05, 0.05, 0.05},
		Diffuse:   mgl32.Vec3{0.8, 0.8, 0.8},
		Specular:  mgl32.Vec3{1.0, 1.0, 1.0},
		Constant:  1.0,
		Linear:    0.09,
		Quadratic: 0.032,
	}
}

// SpotLight is a cone from its entity's world position.
//
// FollowCamera makes it the flashlight: position and direction are taken from
// the camera each frame, and the F key (State.FlashLight) switches it on and
// off. A spotlight that does not follow the camera is placed by its entity and
// is on whenever Enabled.
type SpotLight struct {
	Direction mgl32.Vec3 `yaml:"direction"`

	Ambient  mgl32.Vec3 `yaml:"ambient"`
	Diffuse  mgl32.Vec3 `yaml:"diffuse"`
	Specular mgl32.Vec3 `yaml:"specular"`

	Constant  float32 `yaml:"constant"`
	Linear    float32 `yaml:"linear"`
	Quadratic float32 `yaml:"quadratic"`

	// CutOff and OuterCutOff are half-angles in degrees.
	CutOff      float32 `yaml:"cutOff"`
	OuterCutOff float32 `yaml:"outerCutOff"`

	Enabled      bool `yaml:"enabled"`
	FollowCamera bool `yaml:"followCamera"`
}

// NewSpotLight returns the flashlight the engine used to hardcode.
func NewSpotLight() *SpotLight {
	return &SpotLight{
		Direction:    mgl32.Vec3{0, 0, -1},
		Ambient:      mgl32.Vec3{0.0, 0.0, 0.0},
		Diffuse:      mgl32.Vec3{1.0, 1.0, 1.0},
		Specular:     mgl32.Vec3{1.0, 1.0, 1.0},
		Constant:     1.0,
		Linear:       0.09,
		Quadratic:    0.032,
		CutOff:       12.5,
		OuterCutOff:  15.0,
		Enabled:      true,
		FollowCamera: true,
	}
}

// registerBuiltinComponents makes the light types available to scene files.
func registerBuiltinComponents(r *ComponentRegistry) {
	r.MustRegister("DirectionalLight", func() Component { return NewDirectionalLight() })
	r.MustRegister("PointLight", func() Component { return NewPointLight() })
	r.MustRegister("SpotLight", func() Component { return NewSpotLight() })
}

// placedPointLight pairs a point light with the world position of its entity.
type placedPointLight struct {
	light    *PointLight
	position mgl32.Vec3
}

// placedSpotLight pairs a spot light with its entity's world position.
type placedSpotLight struct {
	light    *SpotLight
	position mgl32.Vec3
}

// lightSet is what one frame's entity walk collects for the shader.
type lightSet struct {
	directional *DirectionalLight
	points      []placedPointLight
	spot        *placedSpotLight

	// droppedPoints counts lights beyond MaxPointLights.
	droppedPoints int
}

// collect gathers the lights attached to one entity.
func (s *lightSet) collect(entity *Entity) {
	for _, component := range entity.components {
		switch light := component.(type) {
		case *DirectionalLight:
			// The shader has a single dirLight slot; first one wins.
			if s.directional == nil {
				s.directional = light
			}
		case *PointLight:
			if len(s.points) >= MaxPointLights {
				s.droppedPoints++
				continue
			}
			s.points = append(s.points, placedPointLight{
				light:    light,
				position: entity.WorldMatrix().Col(3).Vec3(),
			})
		case *SpotLight:
			if s.spot == nil {
				s.spot = &placedSpotLight{
					light:    light,
					position: entity.WorldMatrix().Col(3).Vec3(),
				}
			}
		}
	}
}
