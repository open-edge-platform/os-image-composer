// github.com/microsoft/azurelinux/toolkit/tools/internal/logger/logger.go
package logger

import "go.uber.org/zap"

var global *zap.SugaredLogger

// Init the imagedisc to set the Zap logger once.
func Init(z *zap.SugaredLogger) { global = z }

// Logger() is what diskutils will call. It must return a non‐nil *SugaredLogger.
func Logger() *zap.SugaredLogger {
	if global == nil {
		// In case someone calls CreatePartitions before Init, return a no‐op logger.
		return zap.NewNop().Sugar()
	}
	return global
}
