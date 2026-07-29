// Package components holds the game-side components: behaviours a scene file
// can attach to an entity, as opposed to the engine's built-in lights.
//
// It exists partly to be an example. The engine's component registry and its
// YAML props decoding are general, but nothing was using them, so this package
// is the end-to-end proof: a plain Go struct with yaml-tagged fields, a
// constructor holding its defaults, a name in the registry, and a `type:` line
// in a scene file.
//
// The shape to copy is:
//
//   - exported, yaml-tagged fields for everything a scene file may set,
//   - a constructor that returns the defaults, because the engine builds a
//     component from the factory first and only then decodes props over it, so
//     an omitted property keeps whatever the constructor put there,
//   - one or more of engine.Starter/Updater/FixedUpdater/Destroyer,
//   - a line in Register.
//
// Anything that persists across a save has to be an exported field. Both
// components here keep their accumulated angle in a property rather than in
// private state, so saving mid-motion and reloading puts the entity back exactly
// where it was rather than snapping it to the start of its path.
//
// This is also where a script-backed component would go, if the engine ever
// grows scripting: a LuaScript or similar type reading its file name and
// parameters out of the same props block, implementing the same Updater
// interface, and registered on the line below these two. The registry is the
// only seam it needs — nothing else in the engine would have to change.
package components

import (
	"3d-engine/engine"
)

// Register makes every component in this package available to scene files.
//
// Pass it as engine.Options.RegisterComponents rather than calling it on the
// returned App: engine.New loads the initial scene before it returns, so
// registering afterwards is too late for that scene to name anything here.
//
//	app, err := engine.New(engine.Options{
//	    RegisterComponents: components.Register,
//	})
//
// It returns an error rather than panicking because the only way it can fail is
// a name collision with something else the game registered, which is the
// caller's problem to report.
func Register(registry *engine.ComponentRegistry) error {
	if err := registry.Register("Spinner", func() engine.Component { return NewSpinner() }); err != nil {
		return err
	}
	if err := registry.Register("Orbiter", func() engine.Component { return NewOrbiter() }); err != nil {
		return err
	}
	return nil
}
