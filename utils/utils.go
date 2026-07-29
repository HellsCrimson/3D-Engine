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

// Args is the parsed command line. The caller feeds it into the engine rather
// than the engine reaching for a global.
type Args struct {
	ConfigPath string
	ScenePath  string
	DebugLevel DebugLevel
}

// ParseArgs parses the command line and applies the verbosity to the logger.
func ParseArgs() Args {
	var opts struct {
		Verbose []bool `short:"v" long:"verbose" description:"Show verbose debug information"`
		Config  string `short:"c" long:"config" description:"The path to the config" default:"./config.yml"`
		Scene   string `short:"s" long:"scene" description:"The path to the scene" default:"./scene.yml"`
	}

	_, err := flags.Parse(&opts)
	if err != nil {
		os.Exit(1)
	}

	level := DebugLevel(len(opts.Verbose))
	Logger().SetLevel(level)

	return Args{
		ConfigPath: opts.Config,
		ScenePath:  opts.Scene,
		DebugLevel: level,
	}
}
