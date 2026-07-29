package input

import (
	"fmt"
	"sort"

	"github.com/go-gl/glfw/v3.3/glfw"
)

// KeySource reports whether a physical key is held. Poll reads through this
// rather than touching GLFW directly, which keeps the edge-detection logic
// testable and leaves room for a gamepad or replay source later.
type KeySource interface {
	IsKeyDown(key glfw.Key) bool
}

// Window adapts a GLFW window to KeySource.
type Window struct {
	Window *glfw.Window
}

func (w Window) IsKeyDown(key glfw.Key) bool {
	return w.Window.GetKey(key) == glfw.Press
}

// Action is the name a binding is known by, e.g. "move_forward". Game code and
// config files refer to actions; only the Map knows which keys they mean.
type Action string

// Map holds the bindings and the two frames of state that edge detection needs.
//
// It replaces the [glfw.KeyLast]func() bool callback table, which had three
// problems this fixes: bindings were compiled in so nothing could be rebound,
// every frame polled all ~350 keys, and each toggle re-implemented its own
// `glfw.GetTime() - LastPress >= N` debounce. Debouncing is inherent here —
// JustPressed is a rising edge, so a toggle fires exactly once per physical
// press regardless of frame rate.
type Map struct {
	bindings map[Action][]glfw.Key

	// keys is the set of keys any action binds, so Poll touches only those.
	keys []glfw.Key

	down     map[Action]bool
	previous map[Action]bool

	// alwaysActive survives suppression. Without it, an action bound to
	// "close the panel that is capturing the keyboard" could never fire.
	alwaysActive map[Action]bool
}

func NewMap() *Map {
	return &Map{
		bindings:     map[Action][]glfw.Key{},
		down:         map[Action]bool{},
		previous:     map[Action]bool{},
		alwaysActive: map[Action]bool{},
	}
}

// Bind points an action at one or more keys. Any of them triggers it.
func (m *Map) Bind(action Action, keys ...glfw.Key) {
	m.bindings[action] = append(m.bindings[action], keys...)
	m.rebuildKeySet()
}

// BindNames is Bind with config-file key names.
func (m *Map) BindNames(action Action, names ...string) error {
	keys := make([]glfw.Key, 0, len(names))

	for _, name := range names {
		key, err := ParseKey(name)
		if err != nil {
			return fmt.Errorf("action %q: %w", action, err)
		}
		keys = append(keys, key)
	}

	m.bindings[action] = keys
	m.rebuildKeySet()
	return nil
}

// Rebind replaces an action's keys outright, which is what a config file does
// on top of the engine defaults.
func (m *Map) Rebind(action Action, keys ...glfw.Key) {
	m.bindings[action] = keys
	m.rebuildKeySet()
}

// Keys returns the keys bound to an action.
func (m *Map) Keys(action Action) []glfw.Key {
	return m.bindings[action]
}

// Actions lists every bound action, sorted.
func (m *Map) Actions() []Action {
	actions := make([]Action, 0, len(m.bindings))
	for action := range m.bindings {
		actions = append(actions, action)
	}
	sort.Slice(actions, func(i, j int) bool { return actions[i] < actions[j] })
	return actions
}

// Describe renders an action's binding for display, e.g. "W, Up".
func (m *Map) Describe(action Action) string {
	keys := m.bindings[action]
	if len(keys) == 0 {
		return "unbound"
	}

	label := ""
	for i, key := range keys {
		if i > 0 {
			label += ", "
		}
		label += KeyLabel(key)
	}
	return label
}

// SetAlwaysActive exempts an action from suppression, so it keeps working while
// something else owns the keyboard. Use it sparingly — for the action that
// dismisses whatever is capturing input, which would otherwise trap the user.
func (m *Map) SetAlwaysActive(actions ...Action) {
	for _, action := range actions {
		m.alwaysActive[action] = true
	}
}

// AlwaysActive reports whether an action is exempt from suppression.
func (m *Map) AlwaysActive(action Action) bool {
	return m.alwaysActive[action]
}

// Poll samples the keyboard once per frame. Everything else reads the snapshot
// it produces, so all queries within a frame agree with each other.
//
// suppress blanks the map without losing edge tracking — used while the editor
// overlay owns the keyboard, so typing in a text field cannot also drive the
// camera, and releasing focus does not read as a fresh key press. Actions
// marked with SetAlwaysActive are still evaluated.
func (m *Map) Poll(source KeySource, suppress bool) {
	for action, isDown := range m.down {
		m.previous[action] = isDown
	}

	// Sample each physical key once, even when several actions share it.
	pressed := make(map[glfw.Key]bool, len(m.keys))
	for _, key := range m.keys {
		pressed[key] = source.IsKeyDown(key)
	}

	for action, keys := range m.bindings {
		if suppress && !m.alwaysActive[action] {
			m.down[action] = false
			continue
		}

		state := false
		for _, key := range keys {
			if pressed[key] {
				state = true
				break
			}
		}
		m.down[action] = state
	}
}

// IsDown reports whether the action is held this frame.
func (m *Map) IsDown(action Action) bool {
	return m.down[action]
}

// JustPressed reports the rising edge: true only on the frame the action went
// down. This is what toggles should use.
func (m *Map) JustPressed(action Action) bool {
	return m.down[action] && !m.previous[action]
}

// JustReleased reports the falling edge.
func (m *Map) JustReleased(action Action) bool {
	return !m.down[action] && m.previous[action]
}

func (m *Map) rebuildKeySet() {
	seen := map[glfw.Key]bool{}
	m.keys = m.keys[:0]

	for _, keys := range m.bindings {
		for _, key := range keys {
			if seen[key] {
				continue
			}
			seen[key] = true
			m.keys = append(m.keys, key)
		}
	}
}
