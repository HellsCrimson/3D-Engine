package main

import (
	"3d-engine/object"
	"3d-engine/scene"
	"3d-engine/utils"
	"fmt"
	"sort"
	"sync"

	"github.com/go-gl/mathgl/mgl32"
)

type SceneManager struct {
	mu               sync.Mutex
	currentScenePath string
	currentSceneMode string
	pendingScenePath string
	pendingSceneMode string
	sceneModes       map[string]string
	defaultSceneMode string
	fallbackScene    string
}

func NewSceneManager(config *utils.Config, fallbackScenePath string) *SceneManager {
	sceneModes := map[string]string{}
	defaultMode := ""

	if config != nil {
		for name, path := range config.SceneModes {
			sceneModes[name] = path
		}
		defaultMode = config.DefaultSceneMode
	}

	return &SceneManager{
		sceneModes:       sceneModes,
		defaultSceneMode: defaultMode,
		fallbackScene:    fallbackScenePath,
	}
}

func (sm *SceneManager) LoadScene(scenePath string) error {
	loadedScene, err := scene.Load(scenePath)
	if err != nil {
		return err
	}

	newModels := make([]*object.Model, 0, len(loadedScene.Objects))
	modelID := uint32(0)

	for _, obj := range loadedScene.Objects {
		model := &object.Model{Id: modelID, Name: obj.Name}
		if err := model.LoadScene(obj.Path); err != nil {
			return fmt.Errorf("could not load model %q: %w", obj.Path, err)
		}

		model.Coordinates = mgl32.Vec3{obj.OriginX, obj.OriginY, obj.OriginZ}
		model.Rotation = mgl32.Vec4{obj.RotationX, obj.RotationY, obj.RotationZ, obj.RotationAngle}
		model.Scale = mgl32.Vec3{obj.ScaleX, obj.ScaleY, obj.ScaleZ}
		model.IsStatic = obj.IsStatic

		newModels = append(newModels, model)
		modelID++
	}

	modelsMu.Lock()
	models = newModels
	modelsMu.Unlock()

	sm.mu.Lock()
	sm.currentScenePath = scenePath
	sm.currentSceneMode = sm.resolveModeFromPath(scenePath)
	sm.pendingScenePath = ""
	sm.pendingSceneMode = ""
	sm.mu.Unlock()

	return nil
}

func (sm *SceneManager) CurrentScenePath() string {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	return sm.currentScenePath
}

func (sm *SceneManager) CurrentSceneMode() string {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	return sm.currentSceneMode
}

func (sm *SceneManager) ResolveInitialScenePath() string {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if sm.defaultSceneMode != "" {
		if path, ok := sm.sceneModes[sm.defaultSceneMode]; ok {
			return path
		}
	}
	return sm.fallbackScene
}

func (sm *SceneManager) ListModes() map[string]string {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	result := make(map[string]string, len(sm.sceneModes))
	for name, path := range sm.sceneModes {
		result[name] = path
	}
	return result
}

func (sm *SceneManager) ListModeNames() []string {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	names := make([]string, 0, len(sm.sceneModes))
	for name := range sm.sceneModes {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func (sm *SceneManager) RequestSceneChange(scenePath string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.pendingScenePath = scenePath
	sm.pendingSceneMode = ""
}

func (sm *SceneManager) RequestSceneModeChange(sceneMode string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.pendingSceneMode = sceneMode
	sm.pendingScenePath = ""
}

func (sm *SceneManager) ApplyPendingSceneChange() (bool, error) {
	sm.mu.Lock()
	pending := sm.pendingScenePath
	pendingMode := sm.pendingSceneMode
	sm.mu.Unlock()

	if pendingMode != "" {
		sm.mu.Lock()
		scenePath, ok := sm.sceneModes[pendingMode]
		sm.mu.Unlock()
		if !ok {
			return false, fmt.Errorf("scene mode %q is not configured", pendingMode)
		}
		if err := sm.LoadScene(scenePath); err != nil {
			return false, err
		}
		sm.mu.Lock()
		sm.currentSceneMode = pendingMode
		sm.pendingSceneMode = ""
		sm.pendingScenePath = ""
		sm.mu.Unlock()
		return true, nil
	}

	if pending == "" {
		return false, nil
	}

	if err := sm.LoadScene(pending); err != nil {
		return false, err
	}
	return true, nil
}

func (sm *SceneManager) resolveModeFromPath(scenePath string) string {
	for mode, path := range sm.sceneModes {
		if path == scenePath {
			return mode
		}
	}
	return ""
}
