package utils

import (
	"sync"
)

var lockContext = &sync.Mutex{}

type Context struct {
	DebugLevel DebugLevel
	Wireframe  bool
	FlashLight bool

	ConfigPath string
	ScenePath  string
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

type DebugLevel int

const (
	NoDebug DebugLevel = iota
	Info
	Verbose
)
