package log

import (
	"context"
	"fmt"
)

var (
	currentProvider Logger
)

// Init global loggers
func Init(app string, level Level, opts ...Option) {
	o := defaultLogOption
	for _, opt := range opts {
		opt(&o)
	}

	var err error
	currentProvider, err = create(app, level, o)
	if err != nil {
		panic(fmt.Errorf("default logger init error: %v", err))
	}
}

func Sync() error {
	if currentProvider == nil {
		return nil
	}

	return currentProvider.Sync()
}

// Clone return a new logger instance with fields
func Clone(fields ...Field) Logger {
	return currentProvider.With(fields...)
}

// GetLevel for global Logger
func GetLevel() Level {
	return currentProvider.GetLevel()
}

// SetLevel for global Logger
func SetLevel(level Level) {
	currentProvider.SetLevel(level)
}

// Print method of global Logger
func Print(level Level, msg string, fields ...Field) {
	currentProvider.Print(level, msg, fields...)
}

// Debug ...
func Debug(msg string, fields ...Field) {
	currentProvider.Debug(msg, fields...)
}

// Info ...
func Info(msg string, fields ...Field) {
	currentProvider.Info(msg, fields...)
}

// Warn ...
func Warn(msg string, fields ...Field) {
	currentProvider.Warn(msg, fields...)
}

// Error ...
func Error(msg string, fields ...Field) {
	currentProvider.Error(msg, fields...)
}

// Fatal ...
func Fatal(msg string, fields ...Field) {
	currentProvider.Fatal(msg, fields...)
}

// DPanic ...
func DPanic(msg string, fields ...Field) {
	currentProvider.DPanic(msg, fields...)
}

// Panic ...
func Panic(msg string, fields ...Field) {
	currentProvider.Panic(msg, fields...)
}

// For ...
func For(ctx context.Context) Entry {
	return currentProvider.For(ctx)
}
