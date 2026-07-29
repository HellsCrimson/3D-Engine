package utils

import (
	"errors"
	"fmt"
	"log"
	"os"
	"sync"
)

type DebugLevel int

const (
	NoDebug DebugLevel = iota
	Info
	Verbose
)

type LoggerObject struct {
	*log.Logger

	// level is set once from the command line, before any goroutine starts.
	level DebugLevel
}

var lockLogger = &sync.Mutex{}

var loggerInstance *LoggerObject = nil

func Logger() *LoggerObject {
	if loggerInstance == nil {
		lockLogger.Lock()
		defer lockLogger.Unlock()
		if loggerInstance == nil {
			loggerInstance = &LoggerObject{
				Logger: log.New(os.Stdout, "", log.LstdFlags),
			}
		}
	}
	return loggerInstance
}

func (l *LoggerObject) SetLevel(level DebugLevel) {
	l.level = level
}

func (l *LoggerObject) Level() DebugLevel {
	return l.level
}

// Infoln logs only when -v was passed.
func (l *LoggerObject) Infoln(v ...any) {
	if l.level >= Info {
		l.Println(v...)
	}
}

// Verbosef logs only when -vv was passed.
func (l *LoggerObject) Verbosef(format string, v ...any) {
	if l.level >= Verbose {
		l.Printf(format, v...)
	}
}

// Errorf returns an error rather than logging it, so callers can write
// return nil, utils.Logger().Errorf(...).
func (l *LoggerObject) Errorf(format string, a ...any) error {
	msg := fmt.Sprintf(format, a...)
	return errors.New(msg)
}
