package engine

// This file drives component callbacks. Every phase runs with the world
// exclusively locked, which is what makes transform access from a callback safe
// and structural changes (spawn/despawn/scene load) illegal — those must go
// through App.Defer. See Context for the rule.

// context builds the per-callback context. DeltaTime differs per phase.
func (a *App) context(entity *Entity, deltaTime float32) *Context {
	return &Context{
		App:       a,
		World:     a.World,
		Entity:    entity,
		Camera:    a.Camera,
		Keys:      a.Keys,
		DeltaTime: deltaTime,
	}
}

// startAndUpdateComponents runs Start for freshly attached components, then
// Update for everything. Start is deliberately run for all entities before any
// Update, so a component's first Update sees a fully initialised scene.
func (a *App) startAndUpdateComponents() {
	a.World.Write(func(entities []*Entity) {
		for _, entity := range entities {
			for _, component := range entity.takeUnstarted() {
				if starter, ok := component.(Starter); ok {
					starter.Start(a.context(entity, a.deltaTime))
				}
			}
		}

		for _, entity := range entities {
			for _, component := range entity.components {
				if updater, ok := component.(Updater); ok {
					updater.Update(a.context(entity, a.deltaTime))
				}
			}
		}
	})
}

// fixedUpdateComponents runs the fixed-step callbacks. Callers already hold the
// world write lock.
func (a *App) fixedUpdateComponents(entities []*Entity) {
	for _, entity := range entities {
		for _, component := range entity.components {
			if fixed, ok := component.(FixedUpdater); ok {
				fixed.FixedUpdate(a.context(entity, a.physicsDeltaTime))
			}
		}
	}
}

// destroyComponents runs OnDestroy for one entity. Callers already hold the
// world write lock.
func (a *App) destroyComponents(entity *Entity) {
	for _, component := range entity.components {
		if destroyer, ok := component.(Destroyer); ok {
			destroyer.OnDestroy(a.context(entity, a.deltaTime))
		}
	}
}
