package utils

import (
	"os"
	"reflect"
	"sync"
	"unsafe"
)

var lockContext = &sync.Mutex{}

type Context struct {
	Debug               bool
	Wireframe           bool
	LastWireframeChange float64
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
	if len(os.Args) > 1 {
		if os.Args[1] == "-v" {
			context := GetContext()
			context.Debug = true
		}
	}
}
