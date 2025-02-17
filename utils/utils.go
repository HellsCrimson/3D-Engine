package utils

import (
	"os"
	"reflect"
	"unsafe"

	"github.com/jessevdk/go-flags"
)

func Sizeof[T any]() uintptr {
	var v T
	return unsafe.Sizeof(v)
}

func OffsetOf[T any](fieldName string) (uintptr, error) {
	var v T
	rv := reflect.ValueOf(&v).Elem()
	rt := rv.Type()

	field, ok := rt.FieldByName(fieldName)
	if !ok {
		return 0, Logger().Errorf("field %q not found", fieldName)
	}

	return field.Offset, nil
}

func ParseArgs() {
	var opts struct {
		Verbose []bool `short:"v" long:"verbose" description:"Show verbose debug information"`
		Config  string `short:"c" long:"config" description:"The path to the config" default:"./config.yml"`
		Scene   string `short:"s" long:"scene" description:"The path to the scene" default:"./scene.yml"`
	}

	context := GetContext()

	_, err := flags.Parse(&opts)
	if err != nil {
		os.Exit(1)
	}

	context.DebugLevel = DebugLevel(len(opts.Verbose))

	context.ConfigPath = opts.Config
	context.ScenePath = opts.Scene
}
