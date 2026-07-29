package engine

import "sync"

// World holds the scene's entities. It replaces the models slice and its
// separate mutex that used to sit on the App.
//
// Every exported method locks internally. Callers must not hold an entity
// pointer past the callback that handed it to them and mutate it later — the
// frame loop reads entities under the read lock, so writes have to happen
// inside Mutate or Write.
type World struct {
	mu       sync.RWMutex
	entities []*Entity
	byID     map[uint32]*Entity
	nextID   uint32
}

func NewWorld() *World {
	return &World{byID: map[uint32]*Entity{}}
}

// Read runs fn with the entity list read-locked.
func (w *World) Read(fn func(entities []*Entity)) {
	w.mu.RLock()
	defer w.mu.RUnlock()
	fn(w.entities)
}

// Write runs fn with the entity list write-locked. Use it for bulk edits; for a
// single entity prefer Mutate.
func (w *World) Write(fn func(entities []*Entity)) {
	w.mu.Lock()
	defer w.mu.Unlock()
	fn(w.entities)
}

// Mutate applies fn to the entity with the given id under the write lock and
// reports whether it was found.
func (w *World) Mutate(id uint32, fn func(e *Entity)) bool {
	w.mu.Lock()
	defer w.mu.Unlock()

	entity, ok := w.byID[id]
	if !ok {
		return false
	}
	fn(entity)
	return true
}

// Find returns the first entity with the given name, or nil.
func (w *World) Find(name string) *Entity {
	w.mu.RLock()
	defer w.mu.RUnlock()

	for _, entity := range w.entities {
		if entity.Name == name {
			return entity
		}
	}
	return nil
}

func (w *World) Len() int {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return len(w.entities)
}

// Spawn assigns the entity an id and adds it to the world.
func (w *World) Spawn(e *Entity) *Entity {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.spawn(e)
	return e
}

// Despawn removes the entity with the given id and reports whether it existed.
// It does not release the entity's GPU resources; that arrives with the asset
// cache.
func (w *World) Despawn(id uint32) bool {
	w.mu.Lock()
	defer w.mu.Unlock()

	entity, ok := w.byID[id]
	if !ok {
		return false
	}

	entity.SetParent(nil)
	// Copy first: SetParent edits the parent's child slice as it goes.
	orphans := append([]*Entity(nil), entity.Children()...)
	for _, child := range orphans {
		child.SetParent(nil)
	}

	delete(w.byID, id)
	for i, candidate := range w.entities {
		if candidate == entity {
			w.entities = append(w.entities[:i], w.entities[i+1:]...)
			break
		}
	}
	return true
}

// Replace swaps in a whole new set of entities, as a scene load does. Ids
// restart from zero, which is why a client holding ids across a scene change
// currently targets the wrong entity.
func (w *World) Replace(entities []*Entity) {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.entities = make([]*Entity, 0, len(entities))
	w.byID = make(map[uint32]*Entity, len(entities))
	w.nextID = 0

	for _, entity := range entities {
		w.spawn(entity)
	}
}

// spawn adds an entity. Callers hold the write lock.
func (w *World) spawn(e *Entity) {
	e.ID = w.nextID
	w.nextID++
	w.entities = append(w.entities, e)
	w.byID[e.ID] = e
}
