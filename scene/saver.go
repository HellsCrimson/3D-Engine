package scene

import (
	"bytes"
	"os"
	"path/filepath"

	"3d-engine/utils"

	"gopkg.in/yaml.v3"
)

// Save writes the scene as version 2 YAML.
//
// The write goes to a temporary file in the same directory and is renamed into
// place, so a failure part-way through leaves the previous scene file intact
// rather than truncated. Overwriting the scene you are standing in is the normal
// case for an editor save, and losing it to a half-written file would be the
// worst possible outcome.
func Save(path string, s *Scene) error {
	if s == nil {
		return utils.Logger().Errorf("cannot save a nil scene to %s", path)
	}

	// The version is the writer's, not whatever the struct happened to carry:
	// this build only knows how to emit the current format.
	saved := *s
	saved.Version = CurrentVersion

	if err := validate(&saved, path); err != nil {
		return err
	}

	encoded, err := encode(&saved)
	if err != nil {
		return utils.Logger().Errorf("failed to encode scene %s: %s", path, err)
	}

	directory := filepath.Dir(path)
	temporary, err := os.CreateTemp(directory, ".scene-*.yml")
	if err != nil {
		return utils.Logger().Errorf("failed to create a temporary file next to %s: %s", path, err)
	}
	temporaryPath := temporary.Name()

	// Remove the temporary file on every failure below. Once the rename
	// succeeds there is nothing left at that name and the removal is a no-op.
	defer os.Remove(temporaryPath)

	if _, err := temporary.Write(encoded); err != nil {
		temporary.Close()
		return utils.Logger().Errorf("failed to write scene %s: %s", path, err)
	}
	if err := temporary.Close(); err != nil {
		return utils.Logger().Errorf("failed to close scene %s: %s", path, err)
	}

	if err := os.Rename(temporaryPath, path); err != nil {
		return utils.Logger().Errorf("failed to replace scene %s: %s", path, err)
	}

	return nil
}

// encode renders the scene the way the hand-written scene files are written,
// rather than the way yaml.Marshal would leave it: two-space indentation, and
// vectors on one line.
//
// Going through a node tree is what makes the second part possible. Style is a
// property of a node, so the scene has to exist as nodes before it becomes text,
// and this is also the only point where component props — arbitrary structs the
// engine knows nothing about — can be reached and formatted alongside the
// transforms.
func encode(s *Scene) ([]byte, error) {
	var root yaml.Node
	if err := root.Encode(s); err != nil {
		return nil, err
	}
	compactScalarSequences(&root)

	var out bytes.Buffer
	encoder := yaml.NewEncoder(&out)
	encoder.SetIndent(2)

	if err := encoder.Encode(&root); err != nil {
		encoder.Close()
		return nil, err
	}
	if err := encoder.Close(); err != nil {
		return nil, err
	}

	return out.Bytes(), nil
}

// compactScalarSequences puts sequences of plain values on one line, so a
// position reads as [0, 5, 0] instead of spilling over four.
//
// It keys off the shape rather than the field, because component props are
// user-defined: any struct with a vector in it gets the same treatment as the
// built-in transform without the saver having to know about it. A sequence
// holding anything structured stays in block style, which is what keeps the
// object list readable.
func compactScalarSequences(node *yaml.Node) {
	if node.Kind == yaml.SequenceNode {
		allScalars := true
		for _, child := range node.Content {
			if child.Kind != yaml.ScalarNode {
				allScalars = false
				break
			}
		}
		if allScalars && len(node.Content) > 0 {
			node.Style = yaml.FlowStyle
		}
	}

	for _, child := range node.Content {
		compactScalarSequences(child)
	}
}

// validate rejects scenes Load would refuse to read back. A save that cannot be
// reloaded is not a save, so it is better to fail here, naming the object, than
// to write a file that only breaks on the next load.
func validate(s *Scene, path string) error {
	for i := range s.Objects {
		obj := &s.Objects[i]
		if obj.Model == "" && len(obj.Components) == 0 {
			return utils.Logger().Errorf(
				"cannot save scene %s: object %q has neither a model nor any components, so reloading it would fail",
				path, obj.Name)
		}
	}
	return nil
}
