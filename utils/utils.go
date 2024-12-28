package utils

import (
	"os"
	"reflect"
	"sync"
	"unsafe"

	"github.com/jessevdk/go-flags"
)

var lockContext = &sync.Mutex{}

type Context struct {
	Debug               bool
	Wireframe           bool
	LastWireframeChange float64

	ModelPath string
}

var contextInstance *Context

func GetContext() *Context {
	if contextInstance == nil {
		lockContext.Lock()
		defer lockContext.Unlock()
		if contextInstance == nil {
			contextInstance = &Context{}
		}
	}
	return contextInstance
}

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
		Path    string `short:"p" long:"path" description:"The path to the object" required:"true"`
	}

	context := GetContext()

	_, err := flags.Parse(&opts)
	if err != nil {
		os.Exit(1)
	}

	if len(opts.Verbose) > 0 {
		context.Debug = opts.Verbose[0]
	} else {
		context.Debug = false
	}

	context.ModelPath = opts.Path
}
