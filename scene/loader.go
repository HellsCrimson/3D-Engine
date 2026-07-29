package scene

import (
	"3d-engine/utils"
	"os"

	"gopkg.in/yaml.v3"
)

// CurrentVersion is the scene format this build writes and prefers.
//
// Version 1 is the original flat layout: a model path in `path`, the transform
// spread across originX/scaleY/rotationAngle/... fields, and `isStatic`. It has
// no room for per-component properties, which is why version 2 exists. Files
// without a `version:` key are read as version 1, so existing scenes keep
// loading unchanged.
const CurrentVersion = 2

type Scene struct {
	Version int      `yaml:"version"`
	Objects []Object `yaml:"objects"`
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

type Object struct {
	Name string `yaml:"name"`

	// Version 2 fields.
	Model      string          `yaml:"model"`
	Transform  *TransformSpec  `yaml:"transform"`
	Body       *BodySpec       `yaml:"body"`
	Components []ComponentSpec `yaml:"components"`

	// Version 1 fields, still honoured when the version 2 equivalents are absent.
	Path          string  `yaml:"path"`
	IsStatic      bool    `yaml:"isStatic"`
	OriginX       float32 `yaml:"originX"`
	OriginY       float32 `yaml:"originY"`
	OriginZ       float32 `yaml:"originZ"`
	ScaleX        float32 `yaml:"scaleX"`
	ScaleY        float32 `yaml:"scaleY"`
	ScaleZ        float32 `yaml:"scaleZ"`
	RotationAngle float32 `yaml:"rotationAngle"`
	RotationX     float32 `yaml:"rotationX"`
	RotationY     float32 `yaml:"rotationY"`
	RotationZ     float32 `yaml:"rotationZ"`
}

// ModelPath returns the model to import, preferring the version 2 key.
func (o *Object) ModelPath() string {
	if o.Model != "" {
		return o.Model
	}
	return o.Path
}

// ResolveTransform returns the placement, reading whichever layout the file
// used.
func (o *Object) ResolveTransform() TransformSpec {
	if o.Transform != nil {
		return *o.Transform
	}

	return TransformSpec{
		Position: [3]float32{o.OriginX, o.OriginY, o.OriginZ},
		Rotation: [4]float32{o.RotationX, o.RotationY, o.RotationZ, o.RotationAngle},
		Scale:    [3]float32{o.ScaleX, o.ScaleY, o.ScaleZ},
	}
}

// ResolveStatic returns whether the body is static, from either layout.
func (o *Object) ResolveStatic() bool {
	if o.Body != nil {
		return o.Body.Static
	}
	return o.IsStatic
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

	if scene.Version == 0 {
		scene.Version = 1
	}
	if scene.Version > CurrentVersion {
		return nil, utils.Logger().Errorf(
			"scene %s is version %d but this build understands up to %d",
			path, scene.Version, CurrentVersion)
	}

	for i := range scene.Objects {
		if scene.Objects[i].ModelPath() == "" {
			return nil, utils.Logger().Errorf(
				"scene %s: object %q has no model path", path, scene.Objects[i].Name)
		}
	}

	return scene, nil
}
