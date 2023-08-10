package logger

import (
	"sync"

	"go.uber.org/zap"
)

type Logger interface {
	zap.Logger
	GetNamedLogger(name string) *Log
	Sync()
}

var (
	instance *Log
	once     sync.Once
)

type Log struct {
	Logger *zap.Logger
}

func CreateLoggerInstance() {
	once.Do(func() {
		logger, err := zap.NewProduction()
		if err != nil {
			panic("Failed to initialize logger")
		}
		instance = &Log{
			Logger: logger,
		}
	})
}

func GetNamedLogger(name string) *Log {
	return &Log{
		Logger: instance.Logger.Named(name),
	}
}

func Sync() {
	instance.Logger.Sync()
}
