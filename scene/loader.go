package scene

import (
	"3d-engine/utils"
	"os"

	"gopkg.in/yaml.v3"
)

// CurrentVersion is the scene format this build reads and writes.
//
// Version 1 was the original flat layout — a model path in `path`, the
// transform spread across originX/scaleY/rotationAngle/... fields, and
// `isStatic`. It had no room for per-component properties, so version 2
// replaced it with a nested transform plus `body` and `components` blocks.
// Version 1 is no longer accepted; a file must declare its version.
const CurrentVersion = 2

type Scene struct {
	Version int         `yaml:"version"`
	Skybox  string      `yaml:"skybox"`
	Camera  *CameraSpec `yaml:"camera"`
	Objects []Object    `yaml:"objects"`
}

// CameraSpec is where the camera starts, and where it returns to when the scene
// reloads. It used to be the constant {0, 0, 3} inside resetDynamicState.
type CameraSpec struct {
	Position [3]float32 `yaml:"position"`
	Yaw      float32    `yaml:"yaw"`
	Pitch    float32    `yaml:"pitch"`
}

func (c *CameraSpec) UnmarshalYAML(node *yaml.Node) error {
	type cameraSpec CameraSpec

	decoded := cameraSpec(DefaultCamera())
	if err := node.Decode(&decoded); err != nil {
		return err
	}

	*c = CameraSpec(decoded)
	return nil
}

// DefaultCamera is the placement used when a scene omits the camera block.
func DefaultCamera() CameraSpec {
	return CameraSpec{
		Position: [3]float32{0.0, 0.0, 3.0},
		Yaw:      -90.0,
		Pitch:    0.0,
	}
}

// ResolveCamera returns the scene's camera placement or the default.
func (s *Scene) ResolveCamera() CameraSpec {
	if s.Camera != nil {
		return *s.Camera
	}
	return DefaultCamera()
}

// TransformSpec is the version 2 nested transform. Omitted fields keep the
// defaults set in UnmarshalYAML rather than collapsing to zero — a missing
// `scale` means unit scale, not a scale of zero.
type TransformSpec struct {
	Position [3]float32 `yaml:"position"`
	Rotation [4]float32 `yaml:"rotation"` // XYZ axis, W angle in degrees
	Scale    [3]float32 `yaml:"scale"`
}

func (t *TransformSpec) UnmarshalYAML(node *yaml.Node) error {
	// The alias type avoids recursing back into this method.
	type transformSpec TransformSpec

	decoded := transformSpec{
		Rotation: [4]float32{0, 1, 0, 0},
		Scale:    [3]float32{1, 1, 1},
	}
	if err := node.Decode(&decoded); err != nil {
		return err
	}

	*t = TransformSpec(decoded)
	return nil
}

// BodySpec configures the built-in RigidBody.
type BodySpec struct {
	Static bool `yaml:"static"`
}

// ComponentSpec names a registered component type and carries its properties
// verbatim. Props stays an undecoded node because only the engine's component
// registry knows what Go type to decode it into.
type ComponentSpec struct {
	Type  string    `yaml:"type"`
	Props yaml.Node `yaml:"props"`
}

// HasProps reports whether the spec carried a props block to decode.
func (c *ComponentSpec) HasProps() bool {
	return !c.Props.IsZero()
}

// Object is one scene entity. Model is optional: an object with no model but
// with components is a perfectly good light or logic node.
type Object struct {
	Name       string          `yaml:"name"`
	Model      string          `yaml:"model"`
	Transform  *TransformSpec  `yaml:"transform"`
	Body       *BodySpec       `yaml:"body"`
	Components []ComponentSpec `yaml:"components"`
}

// ResolveTransform returns the placement, defaulting to the identity when the
// object omits the block entirely.
func (o *Object) ResolveTransform() TransformSpec {
	if o.Transform != nil {
		return *o.Transform
	}

	return TransformSpec{
		Rotation: [4]float32{0, 1, 0, 0},
		Scale:    [3]float32{1, 1, 1},
	}
}

// IsStatic reports whether the body should be excluded from integration.
func (o *Object) IsStatic() bool {
	return o.Body != nil && o.Body.Static
}

func Load(path string) (*Scene, error) {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil, utils.Logger().Errorf("scene file does not exist: %s", path)
	}

	fileContent, err := os.ReadFile(path)
	if err != nil {
		return nil, utils.Logger().Errorf("failed to read scene file: %s", err)
	}

	scene := &Scene{}
	if err := yaml.Unmarshal(fileContent, scene); err != nil {
		return nil, utils.Logger().Errorf("failed to parse YAML scene: %s", err)
	}

	if scene.Version != CurrentVersion {
		return nil, utils.Logger().Errorf(
			"scene %s declares version %d; this build reads version %d",
			path, scene.Version, CurrentVersion)
	}

	for i := range scene.Objects {
		obj := &scene.Objects[i]
		if obj.Model == "" && len(obj.Components) == 0 {
			return nil, utils.Logger().Errorf(
				"scene %s: object %q has neither a model nor any components, so it would do nothing",
				path, obj.Name)
		}
	}

	return scene, nil
}
