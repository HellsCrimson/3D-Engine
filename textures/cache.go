package textures

import (
	"fmt"
	"path/filepath"
	"sync"

	"github.com/go-gl/gl/v4.6-core/gl"
)

// entry is one cached GL texture and the number of holders keeping it alive.
type entry struct {
	id          uint32
	transparent bool
	refs        int
}

// cache is process-wide because GL texture names are, and because this engine
// runs a single context on a single thread. Acquire and Release must both be
// called from that thread.
var cache = struct {
	mu      sync.Mutex
	entries map[string]*entry
}{entries: map[string]*entry{}}

// Acquire uploads the texture the first time it is asked for and hands out the
// same GL name afterwards, counting holders. Two models referencing the same
// texture file therefore share one upload, and neither can free it out from
// under the other.
//
// Every Acquire must be paired with a Release.
func Acquire(path string) (id uint32, transparent bool, err error) {
	key, err := cacheKey(path)
	if err != nil {
		return 0, false, err
	}

	cache.mu.Lock()
	defer cache.mu.Unlock()

	if existing, ok := cache.entries[key]; ok {
		existing.refs++
		return existing.id, existing.transparent, nil
	}

	isTransparent := false
	textureID, err := Load(path, &isTransparent)
	if err != nil {
		return 0, false, err
	}

	cache.entries[key] = &entry{id: textureID, transparent: isTransparent, refs: 1}
	return textureID, isTransparent, nil
}

// Release drops one hold on a texture and deletes it from the GPU when the last
// holder lets go. Releasing something that was never acquired is an error
// rather than a silent no-op, since it means the refcounts are already wrong.
func Release(path string) error {
	key, err := cacheKey(path)
	if err != nil {
		return err
	}

	cache.mu.Lock()
	defer cache.mu.Unlock()

	existing, ok := cache.entries[key]
	if !ok {
		return fmt.Errorf("texture %q was released without being acquired", path)
	}

	existing.refs--
	if existing.refs > 0 {
		return nil
	}

	gl.DeleteTextures(1, &existing.id)
	delete(cache.entries, key)
	return nil
}

// Stats reports how many distinct textures are resident and how many holds are
// outstanding. Used by tests and the leak check.
func Stats() (resident int, holds int) {
	cache.mu.Lock()
	defer cache.mu.Unlock()

	for _, existing := range cache.entries {
		resident++
		holds += existing.refs
	}
	return resident, holds
}

func cacheKey(path string) (string, error) {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("could not resolve texture path %q: %w", path, err)
	}
	return absolute, nil
}
