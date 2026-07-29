package engine

import "sync"

// slot is one row of the world's handle table.
type slot struct {
	generation uint32
	// dense indexes into World.entities, or -1 when the slot is vacant.
	dense int
}

// World holds the scene's entities. It replaces the models slice and its
// separate mutex that used to sit on the App.
//
// Entities live in a dense slice so the frame loop can walk them without
// chasing holes, plus a slot table that maps a Handle to its position. Slots
// are recycled through a free list, and each reuse bumps the slot's generation
// so handles to dead entities stop resolving.
//
// Every exported method locks internally. Callers must not hold an entity
// pointer past the callback that handed it to them and mutate it later — the
// frame loop reads entities under the read lock, so writes have to happen
// inside Mutate or Write.
type World struct {
	mu       sync.RWMutex
	entities []*Entity
	slots    []slot
	free     []uint32

	// onDespawn, when set, runs just before an entity leaves the world. The App
	// uses it to fire Destroyer components. It is called with the write lock
	// held, so it must not re-enter the World.
	onDespawn func(e *Entity)
}

func NewWorld() *World {
	return &World{}
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

// Get resolves a handle, returning nil if it refers to a despawned entity or a
// slot that has since been reused.
func (w *World) Get(h Handle) *Entity {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.get(h)
}

// Mutate applies fn to the handle's entity under the write lock and reports
// whether the handle still resolves.
func (w *World) Mutate(h Handle, fn func(e *Entity)) bool {
	w.mu.Lock()
	defer w.mu.Unlock()

	entity := w.get(h)
	if entity == nil {
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

// Spawn adds the entity to the world and stamps it with a fresh handle.
func (w *World) Spawn(e *Entity) *Entity {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.spawn(e)
	return e
}

// Despawn removes the handle's entity and reports whether it was still alive.
// It does not release the entity's GPU resources; that arrives with the asset
// cache.
func (w *World) Despawn(h Handle) bool {
	w.mu.Lock()
	defer w.mu.Unlock()

	entity := w.get(h)
	if entity == nil {
		return false
	}

	if w.onDespawn != nil {
		w.onDespawn(entity)
	}

	entity.SetParent(nil)
	// Copy first: SetParent edits the parent's child slice as it goes.
	orphans := append([]*Entity(nil), entity.Children()...)
	for _, child := range orphans {
		child.SetParent(nil)
	}

	w.despawn(h)
	return true
}

// Replace swaps in a whole new set of entities, as a scene load does. Every
// previously issued handle is invalidated: the slot table is rebuilt with bumped
// generations, so a client still holding ids from the old scene gets a clean
// "not found" instead of silently addressing a different object.
func (w *World) Replace(entities []*Entity) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.onDespawn != nil {
		for _, outgoing := range w.entities {
			w.onDespawn(outgoing)
		}
	}

	// Retire every live slot so its generation moves on.
	for index := range w.slots {
		if w.slots[index].dense >= 0 {
			w.slots[index].dense = -1
			w.slots[index].generation++
			w.free = append(w.free, uint32(index))
		}
	}

	w.entities = make([]*Entity, 0, len(entities))
	for _, entity := range entities {
		w.spawn(entity)
	}
}

// get resolves a handle. Callers hold at least the read lock.
func (w *World) get(h Handle) *Entity {
	if h.IsZero() || h.Index >= uint32(len(w.slots)) {
		return nil
	}

	s := w.slots[h.Index]
	if s.dense < 0 || s.generation != h.Generation {
		return nil
	}
	return w.entities[s.dense]
}

// spawn adds an entity, reusing a retired slot when one is available. Callers
// hold the write lock.
func (w *World) spawn(e *Entity) {
	var index uint32

	if n := len(w.free); n > 0 {
		index = w.free[n-1]
		w.free = w.free[:n-1]
	} else {
		index = uint32(len(w.slots))
		// Generation 0 is never issued, so the zero Handle stays invalid.
		w.slots = append(w.slots, slot{generation: 1, dense: -1})
	}

	w.slots[index].dense = len(w.entities)
	e.handle = Handle{Index: index, Generation: w.slots[index].generation}
	w.entities = append(w.entities, e)
}

// despawn removes an entity by swapping the last one into its place, which
// keeps the dense slice hole-free. Callers hold the write lock.
func (w *World) despawn(h Handle) {
	dense := w.slots[h.Index].dense
	last := len(w.entities) - 1

	if dense != last {
		moved := w.entities[last]
		w.entities[dense] = moved
		w.slots[moved.handle.Index].dense = dense
	}

	w.entities[last] = nil
	w.entities = w.entities[:last]

	w.slots[h.Index].dense = -1
	w.slots[h.Index].generation++
	w.free = append(w.free, h.Index)
}
