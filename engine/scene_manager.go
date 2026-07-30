package engine

import (
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

	// cameraSpawn is where the current scene puts the camera on load and reset.
	cameraSpawn scene.CameraSpec
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

	// Build the new scene before tearing down the old one. If a model fails to
	// import we release only what this attempt acquired and leave the running
	// scene untouched, rather than unloading it and having nothing to show.
	entities := make([]*Entity, 0, len(loadedScene.Objects))

	for i := range loadedScene.Objects {
		obj := &loadedScene.Objects[i]

		spec, err := sm.buildSpec(obj)
		if err != nil {
			sm.app.releaseModels(entities)
			return err
		}

		// One flat list of every entity in the scene, however deep: the tree is
		// in their parent pointers, and World.Replace wants the lot.
		subtree, err := sm.app.BuildTree(spec)
		if err != nil {
			sm.app.releaseModels(entities)
			return err
		}
		entities = append(entities, subtree...)
	}

	// Hold a reference to the outgoing scene's models until after the swap, so
	// a model shared with the incoming scene never drops to zero refs and gets
	// deleted only to be re-imported.
	outgoing := sm.app.currentSceneEntities()

	sm.app.World.Replace(entities)

	sm.app.releaseModels(outgoing)

	if err := sm.app.setSkybox(loadedScene.Skybox); err != nil {
		utils.Logger().Printf("Loading skybox: %v", err)
	}

	sm.mu.Lock()
	sm.currentScenePath = scenePath
	sm.currentSceneMode = sm.resolveModeFromPath(scenePath)
	sm.cameraSpawn = loadedScene.ResolveCamera()
	sm.mu.Unlock()

	return nil
}

// CameraSpawn is the current scene's camera placement.
func (sm *SceneManager) CameraSpawn() scene.CameraSpec {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	return sm.cameraSpawn
}

// buildSpec turns one scene-file object, and everything nested under it, into an
// ObjectSpec. The engine's object API does the building from there, so a scene
// row and an ADD_OBJECT request produce an entity by exactly the same code.
//
// It resolves components but touches no assets, which keeps the GL work in
// BuildTree and makes this half testable on its own.
func (sm *SceneManager) buildSpec(obj *scene.Object) (ObjectSpec, error) {
	transform := obj.ResolveTransform()

	spec := ObjectSpec{
		Name:  obj.Name,
		Model: obj.Model,
		Transform: Transform{
			Position: mgl32.Vec3(transform.Position),
			// The scene file's axis-angle becomes a quaternion here, at the
			// boundary. Nothing past this point deals in axes and degrees.
			Rotation: QuatFromAxisAngle(mgl32.Vec4(transform.Rotation)),
			Scale:    mgl32.Vec3(transform.Scale),
		},
	}

	if obj.Body != nil {
		spec.Body = &RigidBody{Static: obj.Body.Static}
	}

	if obj.Material != nil {
		if obj.Model == "" {
			// There is no renderer to hang it on, so this would be dropped
			// silently — and dropped again by the next save, losing it from the
			// file.
			utils.Logger().Printf("Object %q has a material but no model; ignoring it", obj.Name)
		} else {
			color := mgl32.Vec3(obj.Material.Color)
			spec.BaseColor = &color
		}
	}

	for i := range obj.Components {
		componentSpec := &obj.Components[i]

		component, err := sm.app.Components.New(componentSpec.Type)
		if err != nil {
			return ObjectSpec{}, fmt.Errorf("object %q: %w", obj.Name, err)
		}

		if componentSpec.HasProps() {
			if err := componentSpec.Props.Decode(component); err != nil {
				return ObjectSpec{}, fmt.Errorf("object %q: component %q props: %w",
					obj.Name, componentSpec.Type, err)
			}
		}

		spec.Components = append(spec.Components, component)
	}

	for i := range obj.Children {
		child, err := sm.buildSpec(&obj.Children[i])
		if err != nil {
			return ObjectSpec{}, err
		}
		spec.Children = append(spec.Children, child)
	}

	return spec, nil
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
