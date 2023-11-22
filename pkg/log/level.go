package log

import (
	"fmt"

	"go.uber.org/zap/zapcore"
)

// A Level is a logging priority. Higher levels are more important.
type Level int8

const (
	// DebugLevel logs are typically voluminous, and are usually disabled in
	// production.
	DebugLevel Level = iota - 1
	// InfoLevel is the default logging priority.
	InfoLevel
	// WarnLevel logs are more important than Info, but don't need individual
	// human review.
	WarnLevel
	// ErrorLevel logs are high-priority. If an application is running smoothly,
	// it shouldn't generate any error-level logs.
	ErrorLevel
	// DPanicLevel logs are particularly important errors. In development the
	// logger panics after writing the message.
	DPanicLevel
	// PanicLevel logs a message, then panics.
	PanicLevel
	// FatalLevel logs a message, then calls os.Exit(1).
	FatalLevel
)

func (l Level) String() string {
	switch l {
	case DebugLevel:
		return "debug"
	case InfoLevel:
		return "info"
	case WarnLevel:
		return "warn"
	case ErrorLevel:
		return "error"
	case DPanicLevel:
		return "dpanic"
	case PanicLevel:
		return "panic"
	case FatalLevel:
		return "fatal"
	}

	return "unknown level"
}

func (l Level) zapLevel() zapcore.Level {
	return zapcore.Level(l)
}

func parseZapLevel(level zapcore.Level) Level {
	return Level(level)
}

// ParseLevel copy code from zap
func ParseLevel(raw string) (Level, error) {
	var level Level
	switch raw {
	case "debug", "DEBUG":
		level = DebugLevel
	case "info", "INFO", "": // make the zero value useful
		level = InfoLevel
	case "warn", "WARN":
		level = WarnLevel
	case "error", "ERROR":
		level = ErrorLevel
	case "dpanic", "DPANIC":
		level = DPanicLevel
	case "panic", "PANIC":
		level = PanicLevel
	case "fatal", "FATAL":
		level = FatalLevel
	default:
		return level, fmt.Errorf("invalid level: %s", raw)
	}
	return level, nil
}
