package logger

import (
	"fmt"
	"io"
	"os"
	"strings"
	"sync"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type nopSyncer struct {
	mu     sync.RWMutex
	writer io.Writer
}

func (n *nopSyncer) Write(p []byte) (int, error) {
	n.mu.RLock()
	defer n.mu.RUnlock()
	if n.writer == nil {
		return 0, nil
	}
	return n.writer.Write(p)
}

func (n *nopSyncer) Sync() error {
	return nil // no-op
}

type StatusWriter struct {
	Status chan string
}

func (sw *StatusWriter) Write(p []byte) (int, error) {
	sw.Status <- string(p)
	return len(p), nil
}

var (
	sugarLogger  *zap.SugaredLogger
	baseLogger   *zap.Logger
	atomicLevel  zap.AtomicLevel // This allows dynamic level changes
	once         sync.Once
	stderrSyncer = &nopSyncer{writer: os.Stderr}
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
	core := zapcore.NewCore(encoder, zapcore.AddSync(stderrSyncer), atomicLevel)

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

// ReplaceStderrWriter swaps the current stderr writer used by the logger.
// It returns the previous writer (never nil; defaults to os.Stderr).
func ReplaceStderrWriter(newOut io.Writer) (oldOut io.Writer) {
	if newOut == nil {
		newOut = os.Stderr
	}

	stderrSyncer.mu.Lock()
	defer stderrSyncer.mu.Unlock()

	oldOut = stderrSyncer.writer
	if oldOut == nil {
		oldOut = os.Stderr
	}
	stderrSyncer.writer = newOut
	return
}
