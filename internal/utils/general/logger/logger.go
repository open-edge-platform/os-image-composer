package logger

import (
	"fmt"
	"os"
	"strings"
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
	atomicLevel zap.AtomicLevel // This allows dynamic level changes
	once        sync.Once
)

func initLogger() {
	initLoggerWithLevel("info") // Default level
}

// initLoggerWithLevel initializes the logger with a specific level
func initLoggerWithLevel(level string) {
	// Parse log level
	var zapLevel zapcore.Level
	switch strings.ToLower(level) {
	case "debug":
		zapLevel = zapcore.DebugLevel
	case "info":
		zapLevel = zapcore.InfoLevel
	case "warn", "warning":
		zapLevel = zapcore.WarnLevel
	case "error":
		zapLevel = zapcore.ErrorLevel
	default:
		zapLevel = zapcore.InfoLevel // Default to info
	}

	// Create atomic level for dynamic changes
	atomicLevel = zap.NewAtomicLevelAt(zapLevel)

	cfg := zap.NewDevelopmentConfig()
	cfg.Level = atomicLevel
	cfg.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	cfg.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	cfg.EncoderConfig.EncodeCaller = zapcore.ShortCallerEncoder

	encoder := zapcore.NewConsoleEncoder(cfg.EncoderConfig)
	writer := nopSyncer{os.Stderr}
	core := zapcore.NewCore(encoder, writer, atomicLevel)

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

// InitWithLevel sets up the global zap logger with a specific log level
func InitWithLevel(level string) (*zap.SugaredLogger, func()) {
	once.Do(func() {
		initLoggerWithLevel(level)
	})

	// If logger already exists, just change the level dynamically
	if atomicLevel.Enabled(zapcore.InfoLevel) { // Check if atomicLevel is initialized
		SetLogLevel(level)
	}

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

// SetLogLevel dynamically changes the log level without re-initializing the logger
func SetLogLevel(level string) {
	if atomicLevel == (zap.AtomicLevel{}) {
		return // Not initialized yet
	}

	var zapLevel zapcore.Level
	switch strings.ToLower(level) {
	case "debug":
		zapLevel = zapcore.DebugLevel
	case "info":
		zapLevel = zapcore.InfoLevel
	case "warn", "warning":
		zapLevel = zapcore.WarnLevel
	case "error":
		zapLevel = zapcore.ErrorLevel
	default:
		zapLevel = zapcore.InfoLevel
	}

	atomicLevel.SetLevel(zapLevel)
}
