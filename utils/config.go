package utils

import (
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	RenderDistanceMin float32 `yaml:"renderDistanceMin"`
	RenderDistanceMax float32 `yaml:"renderDistanceMax"`
	Fov               float32 `yaml:"fov"`
	Width             int     `yaml:"width"`
	Height            int     `yaml:"height"`
	CameraSpeed       float32 `yaml:"cameraSpeed"`
	Object            struct {
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
	} `yaml:"object"`
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

	return config, nil
}
