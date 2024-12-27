package utils

import (
	"errors"
	"fmt"
	"log"
	"os"
	"sync"
)

type LoggerObject struct {
	*log.Logger
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

func (l *LoggerObject) Errorf(format string, a ...any) error {
	msg := fmt.Sprintf(format, a...)
	return errors.New(msg)
}
