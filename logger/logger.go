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
	logger *zap.Logger
}

func CreateLoggerInstance() {
	once.Do(func() {
		logger, err := zap.NewDevelopmentConfig().Build()

		if err != nil {
			panic("Failed to initialize logger")
		}
		instance = &Log{
			logger: logger,
		}
	})
}

func GetNamedLogger(name string) *zap.Logger {
	if instance.logger == nil {
		CreateLoggerInstance()
	}
	return instance.logger.Named(name)
}

func Sync() {
	instance.logger.Sync()
}
