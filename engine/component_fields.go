package engine

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/go-gl/mathgl/mgl32"
)

// This file exposes a component's properties generically, so a front-end can
// present and edit them without knowing any component's Go type.
//
// It reflects over exactly the fields a scene file can set — exported, and named
// by their yaml tag — which is not a coincidence: that set is already the
// engine's definition of "what a component's configuration is", enforced by the
// save path. A property you can edit here is a property that survives a save,
// and one that does not appear here would not have been saved either.

// FieldKind is the shape of a component property, which is what a front-end
// needs to choose a widget.
type FieldKind int

const (
	// FieldUnsupported is a property whose type has no editor representation.
	// It is reported rather than hidden, so an unexpectedly missing field is
	// visible instead of silently absent.
	FieldUnsupported FieldKind = iota
	FieldBool
	FieldInt
	FieldFloat
	FieldVec3
	FieldVec4
	FieldString
)

func (k FieldKind) String() string {
	switch k {
	case FieldBool:
		return "bool"
	case FieldInt:
		return "int"
	case FieldFloat:
		return "float"
	case FieldVec3:
		return "vec3"
	case FieldVec4:
		return "vec4"
	case FieldString:
		return "string"
	}
	return "unsupported"
}

// ComponentField is one property, carrying its current value. Only the member
// matching Kind is meaningful.
type ComponentField struct {
	// Name is the yaml name — what a scene file would call this property.
	Name string
	Kind FieldKind

	// GoName is the Go field name, for error messages.
	GoName string

	Bool   bool
	Int    int64
	Float  float32
	Vec3   mgl32.Vec3
	Vec4   mgl32.Vec4
	String string
}

// ComponentInfo is a read-only snapshot of one attached component.
type ComponentInfo struct {
	// Type is the registered scene-file name, or empty if the registry cannot
	// name this component — the same condition that would fail a save.
	Type string

	// GoType names the Go type, so an unregistered component is still
	// identifiable in the UI.
	GoType string

	// Index is the component's position in the entity's list, and is how an edit
	// addresses it.
	Index int

	Fields []ComponentField
}

// ComponentsOf snapshots the components attached to an entity, with their
// current property values. Safe from any goroutine.
func (a *App) ComponentsOf(handle Handle) ([]ComponentInfo, bool) {
	var infos []ComponentInfo

	found := a.World.Mutate(handle, func(entity *Entity) {
		infos = make([]ComponentInfo, 0, len(entity.components))

		for i, component := range entity.components {
			info := ComponentInfo{
				Index:  i,
				GoType: reflect.TypeOf(component).String(),
				Fields: readFields(component),
			}
			if name, ok := a.Components.NameOf(component); ok {
				info.Type = name
			}
			infos = append(infos, info)
		}
	})

	return infos, found
}

// SetComponentField writes one property back.
//
// The component is addressed by index and checked against the type name the
// caller thought it was editing, because a front-end works from a snapshot: if
// the entity's components changed in between, writing to whatever now sits at
// that index would silently edit the wrong thing.
func (a *App) SetComponentField(handle Handle, index int, typeName string, field ComponentField) error {
	var err error

	found := a.World.Mutate(handle, func(entity *Entity) {
		if index < 0 || index >= len(entity.components) {
			err = fmt.Errorf("object %s has no component at index %d", handle, index)
			return
		}

		component := entity.components[index]
		if name, ok := a.Components.NameOf(component); ok && typeName != "" && name != typeName {
			err = fmt.Errorf("component %d on object %s is now %q, not %q",
				index, handle, name, typeName)
			return
		}

		err = writeField(component, field)
	})

	if !found {
		return fmt.Errorf("object %s not found", handle)
	}
	return err
}

// readFields reflects a component's editable properties out.
func readFields(component Component) []ComponentField {
	value := reflect.ValueOf(component)
	for value.Kind() == reflect.Ptr {
		if value.IsNil() {
			return nil
		}
		value = value.Elem()
	}
	if value.Kind() != reflect.Struct {
		return nil
	}

	structType := value.Type()
	fields := make([]ComponentField, 0, structType.NumField())

	for i := 0; i < structType.NumField(); i++ {
		structField := structType.Field(i)
		name, ok := yamlName(structField)
		if !ok {
			continue
		}

		field := ComponentField{Name: name, GoName: structField.Name}
		readValue(&field, value.Field(i))
		fields = append(fields, field)
	}

	return fields
}

// yamlName returns the name a scene file uses for a field, and whether the field
// is part of a component's configuration at all.
//
// Unexported fields and `yaml:"-"` are excluded for the same reason the saver
// skips them: they are not part of what a scene file describes, so editing them
// would change something no save could record.
func yamlName(field reflect.StructField) (string, bool) {
	if field.PkgPath != "" {
		return "", false
	}

	tag := field.Tag.Get("yaml")
	if tag == "-" {
		return "", false
	}

	name, _, _ := strings.Cut(tag, ",")
	if name == "" {
		// yaml's own default when a field carries no tag.
		name = strings.ToLower(field.Name)
	}
	return name, true
}

func readValue(field *ComponentField, value reflect.Value) {
	switch value.Kind() {
	case reflect.Bool:
		field.Kind, field.Bool = FieldBool, value.Bool()
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		field.Kind, field.Int = FieldInt, value.Int()
	case reflect.Float32, reflect.Float64:
		field.Kind, field.Float = FieldFloat, float32(value.Float())
	case reflect.String:
		field.Kind, field.String = FieldString, value.String()
	case reflect.Array:
		// mgl32.Vec3 and Vec4 are named [3]float32 and [4]float32, so matching on
		// shape rather than on the named type picks up a plain array too.
		if value.Type().Elem().Kind() != reflect.Float32 {
			field.Kind = FieldUnsupported
			return
		}
		switch value.Len() {
		case 3:
			field.Kind = FieldVec3
			for i := 0; i < 3; i++ {
				field.Vec3[i] = float32(value.Index(i).Float())
			}
		case 4:
			field.Kind = FieldVec4
			for i := 0; i < 4; i++ {
				field.Vec4[i] = float32(value.Index(i).Float())
			}
		default:
			field.Kind = FieldUnsupported
		}
	default:
		field.Kind = FieldUnsupported
	}
}

// writeField finds the named property and assigns it. Callers hold the world
// write lock.
func writeField(component Component, field ComponentField) error {
	value := reflect.ValueOf(component)
	if value.Kind() != reflect.Ptr || value.IsNil() {
		// A component registered as a value type cannot be edited in place: the
		// reflection would be writing to a copy.
		return fmt.Errorf("component %T is not addressable, so its properties cannot be edited",
			component)
	}
	value = value.Elem()
	if value.Kind() != reflect.Struct {
		return fmt.Errorf("component %T is not a struct", component)
	}

	structType := value.Type()
	for i := 0; i < structType.NumField(); i++ {
		name, ok := yamlName(structType.Field(i))
		if !ok || name != field.Name {
			continue
		}
		return writeValue(value.Field(i), field)
	}

	return fmt.Errorf("component %T has no property %q", component, field.Name)
}

func writeValue(target reflect.Value, field ComponentField) error {
	if !target.CanSet() {
		return fmt.Errorf("property %q cannot be set", field.Name)
	}

	switch field.Kind {
	case FieldBool:
		target.SetBool(field.Bool)
	case FieldInt:
		if target.OverflowInt(field.Int) {
			return fmt.Errorf("property %q cannot hold %d", field.Name, field.Int)
		}
		target.SetInt(field.Int)
	case FieldFloat:
		target.SetFloat(float64(field.Float))
	case FieldString:
		target.SetString(field.String)
	case FieldVec3:
		for i := 0; i < 3; i++ {
			target.Index(i).SetFloat(float64(field.Vec3[i]))
		}
	case FieldVec4:
		for i := 0; i < 4; i++ {
			target.Index(i).SetFloat(float64(field.Vec4[i]))
		}
	default:
		return fmt.Errorf("property %q has no editable type", field.Name)
	}

	return nil
}
