package log

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"os"
	"reflect"
	"runtime"
	"strconv"
	"time"

	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type Field = zap.Field

// Logger defines the interface for a logger.
type Logger interface {
	LevelWriter

	// GetLevel return the current level of the logger.
	GetLevel() Level

	// SetLevel sets the current level of the logger.
	SetLevel(level Level)

	// For creates an Entry within a context scope.
	// we can get trace from this context
	For(ctx context.Context) Entry

	// With some fixed fields
	With(fileds ...Field) Logger

	WithOptions(opts ...ZapOption) Logger
}

// LevelWriter custom level writer
type LevelWriter interface {
	// Print allows user to programmatically change the output level.
	// This function comes handy when you just want to define a level in
	// some interceptor or middleware and you don't bother with the
	// currentProvider's function reference.
	Print(level Level, msg string, fields ...Field)

	// Debug logs a message at DebugLevel. The message includes any fields passed
	// at the log site, as well as any fields accumulated on the logger.
	Debug(msg string, fields ...Field)

	// Info logs a message at InfoLevel. The message includes any fields passed
	// at the log site, as well as any fields accumulated on the logger.
	Info(msg string, fields ...Field)

	// Warn logs a message at WarnLevel. The message includes any fields passed
	// at the log site, as well as any fields accumulated on the logger.
	Warn(msg string, fields ...Field)

	// Error logs a message at ErrorLevel. The message includes any fields passed
	// at the log site, as well as any fields accumulated on the logger.
	Error(msg string, fields ...Field)

	// DPanic logs a message at DPanicLevel. The message includes any fields
	// passed at the log site, as well as any fields accumulated on the logger.
	DPanic(msg string, fields ...Field)

	// Panic logs a message at PanicLevel. The message includes any fields passed
	// at the log site, as well as any fields accumulated on the logger.
	Panic(msg string, fields ...Field)

	// Fatal logs a message at FatalLevel.
	Fatal(msg string, fields ...Field)

	// Sync flushing any buffered log entries. Applications should take care to call Sync before exiting.
	Sync() error
}

// Entry for writer
type Entry interface {
	LevelWriter
}

type ctxEntry struct {
	ctx    context.Context
	Writer LevelWriter
}

type ZapOption = zap.Option

func newCtxEntry(ctx context.Context, writer LevelWriter) *ctxEntry {
	e := &ctxEntry{
		ctx:    ctx,
		Writer: writer,
	}
	return e
}

func (e *ctxEntry) traceFields() []Field {
	spanContext := trace.SpanContextFromContext(e.ctx)
	if !spanContext.IsValid() {
		return nil
	}

	fields := []Field{}
	fields = append(fields, zap.String("trace-id", spanContext.TraceID().String()))
	fields = append(fields, zap.String("span-id", spanContext.SpanID().String()))
	return fields
}

func (e *ctxEntry) mergeTrace(fields []Field) []Field {
	traceFields := e.traceFields()
	if len(traceFields) == 0 {
		return fields
	}
	return append(fields, traceFields...)
}

func (e *ctxEntry) Print(level Level, msg string, fields ...Field) {
	e.Writer.Print(level, msg, e.mergeTrace(fields)...)
}

func (e *ctxEntry) Debug(msg string, fields ...Field) {
	e.Writer.Debug(msg, e.mergeTrace(fields)...)
}

func (e *ctxEntry) Info(msg string, fields ...Field) {
	e.Writer.Info(msg, e.mergeTrace(fields)...)
}

func (e *ctxEntry) Warn(msg string, fields ...Field) {
	e.Writer.Warn(msg, e.mergeTrace(fields)...)
}

func (e *ctxEntry) Fatal(msg string, fields ...Field) {
	e.Writer.Fatal(msg, e.mergeTrace(fields)...)
}

func (e *ctxEntry) Error(msg string, fields ...Field) {
	e.Writer.Error(msg, e.mergeTrace(fields)...)
}

func (e *ctxEntry) DPanic(msg string, fields ...Field) {
	e.Writer.DPanic(msg, e.mergeTrace(fields)...)
}

func (e *ctxEntry) Panic(msg string, fields ...Field) {
	e.Writer.Panic(msg, e.mergeTrace(fields)...)
}

func (e *ctxEntry) Sync() error {
	// relax
	return nil
}

// make sures that CustomLogger implements Logger interface
var _ Logger = (*CustomLogger)(nil)

// CustomLogger implements Logger
type CustomLogger struct {
	app    string
	logger *zap.Logger
	level  zap.AtomicLevel
}

// Sync flushing any buffered log entries. Applications should take care to call Sync before exiting.
func (c *CustomLogger) Sync() error {
	return c.logger.Sync()
}

func (c *CustomLogger) WithOptions(opts ...ZapOption) Logger {
	l := c.clone()
	zl := c.clone().logger.WithOptions(opts...)
	l.logger = zl
	return l
}

// Print message with variable level
func (c *CustomLogger) Print(level Level, msg string, fields ...Field) {
	switch level {
	case DebugLevel:
		c.Debug(msg, fields...)
	case InfoLevel:
		c.Info(msg, fields...)
	case WarnLevel:
		c.Warn(msg, fields...)
	case ErrorLevel:
		c.Error(msg, fields...)
	case DPanicLevel:
		c.DPanic(msg, fields...)
	case PanicLevel:
		c.Panic(msg, fields...)
	case FatalLevel:
		c.Fatal(msg, fields...)
	default:
		c.Info(msg, fields...)
	}
}

func (c *CustomLogger) With(fields ...Field) Logger {
	if len(fields) == 0 {
		return c
	}

	l := c.clone()
	l.logger = l.logger.With(fields...)
	return l
}

// GetLevel ...
func (c *CustomLogger) GetLevel() Level {
	return parseZapLevel(c.level.Level())
}

// SetLevel ...
func (c *CustomLogger) SetLevel(level Level) {
	c.level.SetLevel(level.zapLevel())
}

// Debug ...
func (c *CustomLogger) Debug(msg string, fields ...Field) {
	checkFields(fields...)
	c.logger.Debug(msg, fields...)
}

// Info ...
func (c *CustomLogger) Info(msg string, fields ...Field) {
	checkFields(fields...)
	c.logger.Info(msg, fields...)
}

// Warn ...
func (c *CustomLogger) Warn(msg string, fields ...Field) {
	checkFields(fields...)
	c.logger.Warn(msg, fields...)
}

// Error ...
func (c *CustomLogger) Error(msg string, fields ...Field) {
	checkFields(fields...)
	c.logger.Error(msg, fields...)
}

// Fatal ...
func (c *CustomLogger) Fatal(msg string, fields ...Field) {
	checkFields(fields...)
	c.logger.Fatal(msg, fields...)
}

// DPanic ...
func (c *CustomLogger) DPanic(msg string, fields ...Field) {
	checkFields(fields...)
	c.logger.DPanic(msg, fields...)
}

// Panic ...
func (c *CustomLogger) Panic(msg string, fields ...Field) {
	checkFields(fields...)
	c.logger.Panic(msg, fields...)
}

// For creates an Entry within a context scope.
func (c *CustomLogger) For(ctx context.Context) Entry {
	return newCtxEntry(ctx, c)
}

func (c *CustomLogger) clone() *CustomLogger {
	copied := *c
	return &copied
}

// New a Logger
func New(app string, level Level, opts ...Option) (Logger, error) {
	o := defaultLogOption
	o.skip = 1

	for _, opt := range opts {
		opt(&o)
	}

	return create(app, level, o)
}

func getFunctionName(i interface{}) string {
	return runtime.FuncForPC(reflect.ValueOf(i).Pointer()).Name()
}

func encoderConfigHash(config zapcore.EncoderConfig) (hash string) {
	// we need a reliable way to encode the function field
	vf := reflect.ValueOf(config)
	tf := reflect.TypeOf(config)

	fields := make([]zap.Field, 0, vf.NumField())

	for i := 0; i < tf.NumField(); i++ {
		f := tf.Field(i)
		v := vf.Field(i)
		if f.Type.Kind() == reflect.Func {
			fname := getFunctionName(v.Interface())
			fields = append(fields, zap.String(f.Name, fname))
			continue
		}

		fields = append(fields, zap.Any(f.Name, v.Interface()))
	}

	jsonenc := zapcore.NewJSONEncoder(config)
	buf, err := jsonenc.EncodeEntry(zapcore.Entry{}, fields)
	if err != nil {
		panic(fmt.Errorf("encoderConfigHash while jsonenc.EncodeEntry failed: %w", err))
	}

	h := md5.New()
	h.Write(buf.Bytes())
	b := h.Sum(nil)

	return hex.EncodeToString(b)
}

// panicOnRegisterEncoderError test only
var panicOnRegisterEncoderError = func() bool {
	s, ok := os.LookupEnv("_NERDS_LOG_PANIC_ON_REGISTER_ENCODER_ERROR")
	if !ok {
		return false
	}
	t, _ := strconv.ParseBool(s)
	return t
}

func prepareWrappedJSONEncoder(config zapcore.EncoderConfig) (wrappedEncoderName string) {
	// since each config might be different, so we would create different wrapper names too.
	hash := encoderConfigHash(config)
	wrappedEncoderName = "wrapped_json_" + hash

	// error returned could be safely ignored
	err := zap.RegisterEncoder(wrappedEncoderName, func(config zapcore.EncoderConfig) (zapcore.Encoder, error) {
		jsonenc := zapcore.NewJSONEncoder(config)
		return &wrappedJSONEncoder{
			Encoder: jsonenc,
		}, nil
	})
	if panicOnRegisterEncoderError() && err != nil {
		panic(fmt.Errorf("zap.RegisterEncoder %s: %w", wrappedEncoderName, err))
	}

	return wrappedEncoderName
}

func create(app string, level Level, opt logOption) (Logger, error) {
	// If the final option does not have any output paths, we provide a default one.
	// Otherwise there might be duplicated log output.
	if len(opt.outputPaths) == 0 {
		opt.outputPaths = []string{"stderr"}
	}
	if len(opt.errorOutputPaths) == 0 {
		opt.errorOutputPaths = []string{"stderr"}
	}

	callerKey := "caller"
	if opt.omitCaller {
		callerKey = zapcore.OmitKey
	}
	encoderConfig := zapcore.EncoderConfig{
		LevelKey:       "level",
		NameKey:        "logger",
		CallerKey:      callerKey,
		MessageKey:     "msg",
		StacktraceKey:  "stacktrace",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeLevel:    zapcore.LowercaseLevelEncoder,
		EncodeTime:     zapcore.TimeEncoderOfLayout("2006-01-02T15:04:05.000Z"),
		EncodeDuration: zapcore.SecondsDurationEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder,
	}
	if opt.withTime {
		encoderConfig.TimeKey = "time"
	}
	zapLevel := zap.NewAtomicLevelAt(level.zapLevel())

	// wrap the json encoder on the fly
	encoding := opt.encoding
	if encoding == "json" {
		encoding = prepareWrappedJSONEncoder(encoderConfig)
	}
	if encoding == "console" {
		encoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
		encoderConfig.EncodeTime = func(t time.Time, enc zapcore.PrimitiveArrayEncoder) {
			enc.AppendString(t.Format(time.Kitchen))
		}
	}

	config := zap.Config{
		Level:       zapLevel,
		Development: opt.development,
		Sampling: &zap.SamplingConfig{
			Initial:    100,
			Thereafter: 100,
		},
		Encoding:         encoding,
		EncoderConfig:    encoderConfig,
		OutputPaths:      opt.outputPaths,
		ErrorOutputPaths: opt.errorOutputPaths,
	}

	var (
		zapLogger *zap.Logger
		err       error
	)
	zapLogger, err = config.Build(zap.AddCallerSkip(opt.skip),
		zap.AddStacktrace(zapcore.DPanicLevel),
		zap.Fields(zap.String("app", app)))

	if err != nil {
		return nil, fmt.Errorf("zap init error: %v", err)
	}

	if len(opt.fields) > 0 {
		zapLogger = zapLogger.With(opt.fields...)
	}
	lg := &CustomLogger{app: app, logger: zapLogger, level: zapLevel}

	return lg, nil
}
