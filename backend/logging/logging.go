// Package logging sets up structured, rotating file logging for Slipstream.
package logging

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"gopkg.in/natefinch/lumberjack.v2"
)

// LogDir returns the directory Init writes rotating logs to for appName —
// %LocalAppData%\<appName>\logs\, falling back to the system temp dir if
// LOCALAPPDATA isn't set. Exported so callers (e.g. an "open logs folder"
// UI action) can find it without duplicating this path logic.
func LogDir(appName string) string {
	localAppData := os.Getenv("LOCALAPPDATA")
	if localAppData == "" {
		localAppData = os.TempDir()
	}
	return filepath.Join(localAppData, appName, "logs")
}

// Init configures a JSON structured logger that writes to a rotating log
// file under %LocalAppData%\<appName>\logs\. It returns the logger, a
// close function to flush/release the log file, and any setup error.
func Init(appName string) (*slog.Logger, func(), error) {
	logDir := LogDir(appName)
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		return nil, func() {}, fmt.Errorf("create log directory: %w", err)
	}

	rotator := &lumberjack.Logger{
		Filename:   filepath.Join(logDir, "slipstream.log"),
		MaxSize:    10, // megabytes
		MaxBackups: 5,
		MaxAge:     28, // days
		Compress:   true,
	}

	handler := slog.NewJSONHandler(rotator, &slog.HandlerOptions{Level: slog.LevelInfo})
	logger := slog.New(handler)
	slog.SetDefault(logger)

	return logger, func() { _ = rotator.Close() }, nil
}
