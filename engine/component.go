package engine

import (
	"fmt"
	"sort"
	"sync"

	"3d-engine/camera"
	"3d-engine/input"
)

// Component is anything attached to an entity. There is no required method:
// implement whichever of the lifecycle interfaces below you need, and the
// engine will call them.
//
//	type Spinner struct{ Speed float32 }
//
//	func (s *Spinner) Update(ctx *engine.Context) {
//	    r := ctx.Entity.Rotation()
//	    ctx.Entity.SetRotation(mgl32.Vec4{0, 1, 0, r.W() + s.Speed*ctx.DeltaTime})
//	}
type Component interface{}

// Starter runs once, before the component's first Update.
type Starter interface {
	Start(ctx *Context)
}

// Updater runs every rendered frame. ctx.DeltaTime is the frame time.
type Updater interface {
	Update(ctx *Context)
}

// FixedUpdater runs on the fixed physics tick, alongside the built-in physics
// step. ctx.DeltaTime is the fixed step, not the frame time.
type FixedUpdater interface {
	FixedUpdate(ctx *Context)
}

// Destroyer runs when the entity is despawned or the scene is replaced.
type Destroyer interface {
	OnDestroy(ctx *Context)
}

// Context is what a component sees during a lifecycle callback.
//
// Callbacks run with the world exclusively locked, so reading and writing any
// entity's transform is safe. Structural changes are not: calling World.Spawn,
// World.Despawn or a scene change from inside a callback deadlocks. Queue those
// with App.Defer, which runs them at the start of the next frame.
type Context struct {
	App    *App
	World  *World
	Entity *Entity
	Camera *camera.Camera

	// Input is the action map, so a component asks for "jump" rather than for
	// a specific key.
	Input *input.Map

	// DeltaTime is the frame time in Update and the fixed step in FixedUpdate.
	DeltaTime float32
}

// ComponentFactory builds a zero-valued component for the registry to populate
// from scene data.
type ComponentFactory func() Component

// ComponentRegistry maps the type names used in scene files to Go constructors.
// A game registers its components at startup:
//
//	app.Components.Register("Spinner", func() engine.Component { return &Spinner{} })
type ComponentRegistry struct {
	mu    sync.RWMutex
	types map[string]ComponentFactory
}

func NewComponentRegistry() *ComponentRegistry {
	return &ComponentRegistry{types: map[string]ComponentFactory{}}
}

// Register makes a component type available to scene files. Registering the
// same name twice is an error rather than a silent overwrite, since the second
// registration would change what every existing scene file means.
func (r *ComponentRegistry) Register(name string, factory ComponentFactory) error {
	if name == "" {
		return fmt.Errorf("component type name must not be empty")
	}
	if factory == nil {
		return fmt.Errorf("component type %q needs a factory", name)
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.types[name]; exists {
		return fmt.Errorf("component type %q is already registered", name)
	}
	r.types[name] = factory
	return nil
}

// MustRegister is Register for setup code that should fail loudly.
func (r *ComponentRegistry) MustRegister(name string, factory ComponentFactory) {
	if err := r.Register(name, factory); err != nil {
		panic(err)
	}
}

// New builds a component by registered name.
func (r *ComponentRegistry) New(name string) (Component, error) {
	r.mu.RLock()
	factory, ok := r.types[name]
	r.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("unknown component type %q (registered: %v)", name, r.Names())
	}

	component := factory()
	if component == nil {
		return nil, fmt.Errorf("factory for component type %q returned nil", name)
	}
	return component, nil
}

// Names lists the registered type names, sorted.
func (r *ComponentRegistry) Names() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0, len(r.types))
	for name := range r.types {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// GetComponent returns the first component on the entity assignable to T.
// It is a function rather than a method because Go methods cannot be generic.
//
//	spinner, ok := engine.GetComponent[*Spinner](entity)
func GetComponent[T Component](e *Entity) (T, bool) {
	for _, component := range e.components {
		if typed, ok := component.(T); ok {
			return typed, true
		}
	}

	var zero T
	return zero, false
}
