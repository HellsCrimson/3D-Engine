package scene

import (
	"3d-engine/utils"
	"os"

	"gopkg.in/yaml.v3"
)

type Scene struct {
	Objects []Object `yaml:"objects"`
}

type Object struct {
	Path          string  `yaml:"path"`
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

	return scene, nil
}
