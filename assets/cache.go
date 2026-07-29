// Package assets keeps imported models resident and shared, so loading the same
// file twice costs one import, and unloading a scene actually frees the GPU
// memory it was holding.
package assets

import (
	"fmt"
	"path/filepath"
	"sync"

	"3d-engine/object"
	"3d-engine/utils"
)

type modelEntry struct {
	model *object.Model
	refs  int
}

// Cache hands out shared models by path and counts holders. It is the only
// thing that calls Model.Delete, which is what makes the release path
// single-owner and therefore safe.
//
// Everything here uploads to or frees from the GPU, so Acquire and Release must
// be called from the frame-loop goroutine. The mutex guards against a caller
// that queued work from elsewhere, not against concurrent GL use.
type Cache struct {
	mu     sync.Mutex
	models map[string]*modelEntry
}

func NewCache() *Cache {
	return &Cache{models: map[string]*modelEntry{}}
}

// Acquire imports the model the first time it is asked for and returns the same
// instance afterwards. Every Acquire must be paired with a Release.
func (c *Cache) Acquire(path string) (*object.Model, error) {
	key, err := cacheKey(path)
	if err != nil {
		return nil, err
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if existing, ok := c.models[key]; ok {
		existing.refs++
		utils.Logger().Infoln("Reusing cached model:", path)
		return existing.model, nil
	}

	model := &object.Model{}
	if err := model.Import(path); err != nil {
		return nil, err
	}

	c.models[key] = &modelEntry{model: model, refs: 1}
	return model, nil
}

// Release drops one hold and deletes the model's GPU resources when the last
// holder lets go.
func (c *Cache) Release(model *object.Model) error {
	if model == nil {
		return nil
	}

	key, err := cacheKey(model.Path)
	if err != nil {
		return err
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	existing, ok := c.models[key]
	if !ok {
		return fmt.Errorf("model %q was released without being acquired", model.Path)
	}

	existing.refs--
	if existing.refs > 0 {
		return nil
	}

	existing.model.Delete()
	delete(c.models, key)
	utils.Logger().Infoln("Unloaded model:", model.Path)
	return nil
}

// Stats reports how many distinct models are resident and how many holds are
// outstanding.
func (c *Cache) Stats() (resident int, holds int) {
	c.mu.Lock()
	defer c.mu.Unlock()

	for _, existing := range c.models {
		resident++
		holds += existing.refs
	}
	return resident, holds
}

func cacheKey(path string) (string, error) {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("could not resolve model path %q: %w", path, err)
	}
	return absolute, nil
}
