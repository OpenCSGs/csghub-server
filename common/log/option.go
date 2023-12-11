package log

var defaultLogOption = logOption{
	skip:             2,
	omitCaller:       false,
	outputPaths:      []string{},
	errorOutputPaths: []string{},
	development:      false,
	encoding:         "json",
	fields:           []Field{},
	withTime:         true,
}

type logOption struct {
	skip             int
	omitCaller       bool
	outputPaths      []string
	errorOutputPaths []string
	encoding         string
	fields           []Field
	development      bool
	withTime         bool
}

// Option allow user to alter default options while initializing a logger
type Option func(opt *logOption)

// WithSkip set caller skip
func WithSkip(skip int) Option {
	return func(opt *logOption) {
		opt.skip = skip
	}
}

// WithEncoding set encoding
func WithEncoding(encoding string) Option {
	return func(opt *logOption) {
		opt.encoding = encoding
	}
}

// WithOmitCaller will omit the caller info, useful when it comes to the AccessLogger case
func WithOmitCaller(b bool) Option {
	return func(opt *logOption) {
		opt.omitCaller = b
	}
}

// WithOutputPaths rewrite output paths. Default is stderr.
func WithOutputPaths(paths ...string) Option {
	return func(opt *logOption) {
		opt.outputPaths = paths
	}
}

// WithErrorOutputPaths rewrite error output paths. Default is stderr.
func WithErrorOutputPaths(paths ...string) Option {
	return func(opt *logOption) {
		opt.errorOutputPaths = paths
	}
}

// WithTimeEnable not to print time if disable, is very useful for testing
func WithTimeEnable(e bool) Option {
	return func(opt *logOption) {
		opt.withTime = e
	}
}

// WithDevelopment puts the logger in development mode, which changes the
// behavior of DPanicLevel and takes stack traces more liberally.
// If the logger is in development mode, it then panics (DPanic means
// "development panic"). This is useful for catching errors that are
// recoverable, but shouldn't ever happen.
// So we encourage users to enable this option in development mode.
func WithDevelopment(t bool) Option {
	return func(opt *logOption) {
		opt.development = t
	}
}

// WithFields add global fields to the logger
func WithFields(fields ...Field) Option {
	return func(opt *logOption) {
		opt.fields = fields
	}
}
