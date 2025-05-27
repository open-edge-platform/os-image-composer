package utils

import (
	"fmt"
	"os"
	"sync"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type nopSyncer struct {
	writer *os.File
}

func (n nopSyncer) Write(p []byte) (int, error) {
	return n.writer.Write(p)
}

func (n nopSyncer) Sync() error {
	return nil // no-op
}

var (
	sugarLogger *zap.SugaredLogger
	baseLogger  *zap.Logger
	once        sync.Once
)

func initLogger() {
	cfg := zap.NewDevelopmentConfig()
	cfg.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	cfg.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	cfg.EncoderConfig.EncodeCaller = zapcore.ShortCallerEncoder

	encoder := zapcore.NewConsoleEncoder(cfg.EncoderConfig)
	writer := nopSyncer{os.Stderr}
	core := zapcore.NewCore(encoder, writer, cfg.Level)

	opts := []zap.Option{
		zap.AddCaller(),
		zap.Development(),
		zap.AddStacktrace(zapcore.ErrorLevel),
	}

	baseLogger = zap.New(core, opts...)
	sugarLogger = baseLogger.Sugar()
}

// Init sets up the global zap logger and installs it as the zap global logger.
// It returns the sugared logger and a cleanup function that must be deferred.
func Init() (*zap.SugaredLogger, func()) {
	once.Do(initLogger)

	if baseLogger == nil {
		panic("logger initialization failed: baseLogger is nil")
	}

	zap.ReplaceGlobals(baseLogger)

	cleanup := func() {
		if err := baseLogger.Sync(); err != nil {
			fmt.Fprintf(os.Stderr, "error syncing logger: %v\n", err)
		}
	}

	return sugarLogger, cleanup
}
func Logger() *zap.SugaredLogger {
	once.Do(initLogger)
	return sugarLogger
}
func With(args ...interface{}) *zap.SugaredLogger {
	return Logger().With(args...)
}
