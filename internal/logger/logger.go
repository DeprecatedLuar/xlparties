// Package logger provides structured, leveled logging for bot actions and
// events (commands, channel lifecycle, voice-state transitions). All other
// packages log through here instead of the stdlib "log" package, so output
// format and level stay consistent.
package logger

import (
	"log/slog"
	"os"
)

var base = slog.New(slog.NewTextHandler(os.Stderr, nil))

func Info(msg string, args ...any) {
	base.Info(msg, args...)
}

func Warn(msg string, args ...any) {
	base.Warn(msg, args...)
}

func Error(msg string, args ...any) {
	base.Error(msg, args...)
}

// Fatal logs at error level then exits the process, mirroring log.Fatal for
// unrecoverable startup failures.
func Fatal(msg string, args ...any) {
	base.Error(msg, args...)
	os.Exit(1)
}
