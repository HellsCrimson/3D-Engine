package engine

import (
	"3d-engine/object"
	"3d-engine/scene"
	"3d-engine/utils"
	"fmt"
	"sort"
	"sync"

	"github.com/go-gl/mathgl/mgl32"
)

// SceneManager owns the current scene selection. Because LoadScene uploads
// meshes and textures to the GPU, changes requested from other goroutines are
// queued onto the frame loop through App.Defer.
type SceneManager struct {
	app *App

	mu               sync.Mutex
	currentScenePath string
	currentSceneMode string
	sceneModes       map[string]string
	defaultSceneMode string
	fallbackScene    string

	// requested* hold the latest pending request. Only one command is ever in
	// flight, so spamming scene changes during a slow load still loads once,
	// with the newest request winning.
	requestQueued bool
	requestedPath string
	requestedMode string
}

func NewSceneManager(app *App, config *utils.Config, fallbackScenePath string) *SceneManager {
	sceneModes := map[string]string{}
	defaultMode := ""

	if config != nil {
		for name, path := range config.SceneModes {
			sceneModes[name] = path
		}
		defaultMode = config.DefaultSceneMode
	}

	return &SceneManager{
		app:              app,
		sceneModes:       sceneModes,
		defaultSceneMode: defaultMode,
		fallbackScene:    fallbackScenePath,
	}
}

// LoadScene imports every object in the scene file and swaps it in. It uploads
// to the GPU, so it must only be called from the goroutine running the frame
// loop.
func (sm *SceneManager) LoadScene(scenePath string) error {
	loadedScene, err := scene.Load(scenePath)
	if err != nil {
		return err
	}

	entities := make([]*Entity, 0, len(loadedScene.Objects))

	for i := range loadedScene.Objects {
		obj := &loadedScene.Objects[i]

		entity, err := sm.buildEntity(obj)
		if err != nil {
			return err
		}
		entities = append(entities, entity)
	}

	sm.app.World.Replace(entities)

	sm.mu.Lock()
	sm.currentScenePath = scenePath
	sm.currentSceneMode = sm.resolveModeFromPath(scenePath)
	sm.mu.Unlock()

	return nil
}

// buildEntity turns one scene-file object into an entity: the model asset, the
// transform, the built-in body, and whatever registered components the file
// asked for.
func (sm *SceneManager) buildEntity(obj *scene.Object) (*Entity, error) {
	modelPath := obj.ModelPath()

	model := &object.Model{}
	if err := model.Import(modelPath); err != nil {
		return nil, fmt.Errorf("could not load model %q: %w", modelPath, err)
	}

	transform := obj.ResolveTransform()
	entity := NewEntity(obj.Name)
	entity.SetTransform(Transform{
		Position: mgl32.Vec3(transform.Position),
		Rotation: mgl32.Vec4(transform.Rotation),
		Scale:    mgl32.Vec3(transform.Scale),
	})
	entity.Renderer = &MeshRenderer{Model: model}
	entity.Body = &RigidBody{Static: obj.ResolveStatic()}

	for i := range obj.Components {
		spec := &obj.Components[i]

		component, err := sm.app.Components.New(spec.Type)
		if err != nil {
			return nil, fmt.Errorf("object %q: %w", obj.Name, err)
		}

		if spec.HasProps() {
			if err := spec.Props.Decode(component); err != nil {
				return nil, fmt.Errorf("object %q: component %q props: %w", obj.Name, spec.Type, err)
			}
		}

		entity.AddComponent(component)
	}

	return entity, nil
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

// RequestSceneChange queues a scene load onto the frame loop. Safe to call from
// any goroutine.
func (sm *SceneManager) RequestSceneChange(scenePath string) {
	sm.request(scenePath, "")
}

// RequestSceneModeChange resolves the mode and queues its scene. Unlike the
// path variant it can fail up front, so an unknown mode is reported to the
// caller instead of only showing up in the engine log a frame later.
func (sm *SceneManager) RequestSceneModeChange(sceneMode string) error {
	sm.mu.Lock()
	scenePath, ok := sm.sceneModes[sceneMode]
	sm.mu.Unlock()

	if !ok {
		return fmt.Errorf("scene mode %q is not configured", sceneMode)
	}

	sm.request(scenePath, sceneMode)
	return nil
}

func (sm *SceneManager) request(scenePath, sceneMode string) {
	sm.mu.Lock()
	sm.requestedPath = scenePath
	sm.requestedMode = sceneMode
	alreadyQueued := sm.requestQueued
	sm.requestQueued = true
	sm.mu.Unlock()

	// A command is already in flight and will pick up the newest request, so
	// queueing another would just load twice.
	if alreadyQueued {
		return
	}

	sm.app.Defer(sm.applyRequest)
}

func (sm *SceneManager) applyRequest(a *App) error {
	sm.mu.Lock()
	scenePath := sm.requestedPath
	sceneMode := sm.requestedMode
	sm.requestedPath = ""
	sm.requestedMode = ""
	sm.requestQueued = false
	sm.mu.Unlock()

	if scenePath == "" {
		return nil
	}

	if err := sm.LoadScene(scenePath); err != nil {
		return fmt.Errorf("failed to switch scene: %w", err)
	}

	if sceneMode != "" {
		sm.mu.Lock()
		sm.currentSceneMode = sceneMode
		sm.mu.Unlock()
	}

	utils.Logger().Printf("Switched scene to %s", scenePath)
	a.resetDynamicState()
	return nil
}

func (sm *SceneManager) resolveModeFromPath(scenePath string) string {
	for mode, path := range sm.sceneModes {
		if path == scenePath {
			return mode
		}
	}
	return ""
}
