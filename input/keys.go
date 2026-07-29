// Package input maps physical keys to named actions, so bindings live in
// config rather than in the code that reacts to them.
package input

import (
	"fmt"
	"sort"
	"strings"

	"github.com/go-gl/glfw/v3.3/glfw"
)

// keyNames is the config-file spelling of every bindable key. Names are matched
// case-insensitively, so "space", "Space" and "SPACE" are the same key.
var keyNames = map[string]glfw.Key{}

// keyLabels is the reverse lookup, for error messages and for showing a binding
// back to the user.
var keyLabels = map[glfw.Key]string{}

func register(name string, key glfw.Key) {
	keyNames[strings.ToLower(name)] = key
	if _, exists := keyLabels[key]; !exists {
		keyLabels[key] = name
	}
}

func init() {
	for letter := 'A'; letter <= 'Z'; letter++ {
		register(string(letter), glfw.KeyA+glfw.Key(letter-'A'))
	}
	for digit := '0'; digit <= '9'; digit++ {
		register(string(digit), glfw.Key0+glfw.Key(digit-'0'))
	}
	for n := 1; n <= 25; n++ {
		register(fmt.Sprintf("F%d", n), glfw.KeyF1+glfw.Key(n-1))
	}

	specials := map[string]glfw.Key{
		"Space":        glfw.KeySpace,
		"Escape":       glfw.KeyEscape,
		"Enter":        glfw.KeyEnter,
		"Tab":          glfw.KeyTab,
		"Backspace":    glfw.KeyBackspace,
		"Insert":       glfw.KeyInsert,
		"Delete":       glfw.KeyDelete,
		"Home":         glfw.KeyHome,
		"End":          glfw.KeyEnd,
		"PageUp":       glfw.KeyPageUp,
		"PageDown":     glfw.KeyPageDown,
		"Up":           glfw.KeyUp,
		"Down":         glfw.KeyDown,
		"Left":         glfw.KeyLeft,
		"Right":        glfw.KeyRight,
		"LeftShift":    glfw.KeyLeftShift,
		"RightShift":   glfw.KeyRightShift,
		"LeftControl":  glfw.KeyLeftControl,
		"RightControl": glfw.KeyRightControl,
		"LeftAlt":      glfw.KeyLeftAlt,
		"RightAlt":     glfw.KeyRightAlt,
		"Minus":        glfw.KeyMinus,
		"Equal":        glfw.KeyEqual,
		"Comma":        glfw.KeyComma,
		"Period":       glfw.KeyPeriod,
		"Slash":        glfw.KeySlash,
		"Semicolon":    glfw.KeySemicolon,
		"Apostrophe":   glfw.KeyApostrophe,
		"GraveAccent":  glfw.KeyGraveAccent,
		"LeftBracket":  glfw.KeyLeftBracket,
		"RightBracket": glfw.KeyRightBracket,
		"Backslash":    glfw.KeyBackslash,
	}
	for name, key := range specials {
		register(name, key)
	}
}

// ParseKey resolves a config-file key name.
func ParseKey(name string) (glfw.Key, error) {
	key, ok := keyNames[strings.ToLower(strings.TrimSpace(name))]
	if !ok {
		return glfw.KeyUnknown, fmt.Errorf("unknown key %q", name)
	}
	return key, nil
}

// KeyLabel returns the canonical name of a key.
func KeyLabel(key glfw.Key) string {
	if label, ok := keyLabels[key]; ok {
		return label
	}
	return fmt.Sprintf("key(%d)", int(key))
}

// KeyNames lists every bindable key name, sorted. Used in error messages.
func KeyNames() []string {
	names := make([]string, 0, len(keyLabels))
	for _, label := range keyLabels {
		names = append(names, label)
	}
	sort.Strings(names)
	return names
}
