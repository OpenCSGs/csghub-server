package log

import (
	"fmt"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func checkFields(fields ...Field) {
	for _, f := range fields {
		checkField(f)
	}
}

func checkField(field Field) {
	// field except skip type will be checked
	if field.Type == zapcore.SkipType {
		return
	}

	if field.Key == "" {
		panic(fmt.Errorf("empty field key is not allowed, type %d", field.Type))
	}
}

// Binary constructs a field that carries an opaque binary blob.
//
// Binary data is serialized in an encoding-appropriate format. For example,
// zap's JSON encoder base64-encodes binary blobs. To log UTF-8 encoded text,
// use ByteString.
func Binary(key string, val []byte) Field {
	return zap.Binary(key, val)
}

// BinaryString constructs a field that carries UTF-8 encoded text as a []byte.
// To log opaque binary blobs (which aren't necessarily valid UTF-8), use
// Binary.
func BinaryString(key string, val []byte) Field {
	return zap.ByteString(key, val)
}

// Bool constructs a field that carries a bool.
func Bool(key string, val bool) Field {
	return zap.Bool(key, val)
}

func nilField(key string) Field { return Reflect(key, nil) }

// Boolp constructs a field that carries a *bool. The returned Field will safely
// and explicitly represent `nil` when appropriate.
func Boolp(key string, val *bool) Field {
	if val == nil {
		return nilField(key)
	}
	return zap.Bool(key, *val)
}

// ByteString constructs a field that carries UTF-8 encoded text as a []byte.
// To log opaque binary blobs (which aren't necessarily valid UTF-8), use
// Binary.
func ByteString(key string, val []byte) Field {
	return zap.ByteString(key, val)
}

// Complex128 constructs a field that carries a complex number. Unlike most
// numeric fields, this costs an allocation (to convert the complex128 to
// interface{}).
func Complex128(key string, val complex128) Field {
	return zap.Complex128(key, val)
}

// Complex128p constructs a field that carries a *complex128. The returned Field will safely
// and explicitly represent `nil` when appropriate.
func Complex128p(key string, val *complex128) Field {
	if val == nil {
		return nilField(key)
	}
	return Complex128(key, *val)
}

// Complex64 constructs a field that carries a complex number. Unlike most
// numeric fields, this costs an allocation (to convert the complex64 to
// interface{}).
func Complex64(key string, val complex64) Field {
	return zap.Complex64(key, val)
}

// Complex64p constructs a field that carries a *complex64. The returned Field will safely
// and explicitly represent `nil` when appropriate.
func Complex64p(key string, val *complex64) Field {
	if val == nil {
		return nilField(key)
	}
	return Complex64(key, *val)
}

// Float64 constructs a field that carries a float64. The way the
// floating-point value is represented is encoder-dependent, so marshaling is
// necessarily lazy.
func Float64(key string, val float64) Field {
	return zap.Float64(key, val)
}

// Float64p constructs a field that carries a *float64. The returned Field will safely
// and explicitly represent `nil` when appropriate.
func Float64p(key string, val *float64) Field {
	if val == nil {
		return nilField(key)
	}
	return Float64(key, *val)
}

// Float32 constructs a field that carries a float32. The way the
// floating-point value is represented is encoder-dependent, so marshaling is
// necessarily lazy.
func Float32(key string, val float32) Field {
	return zap.Float32(key, val)
}

// Float32p constructs a field that carries a *float32. The returned Field will safely
// and explicitly represent `nil` when appropriate.
func Float32p(key string, val *float32) Field {
	if val == nil {
		return nilField(key)
	}
	return Float32(key, *val)
}

// Int constructs a field with the given key and value.
func Int(key string, val int) Field {
	return zap.Int(key, val)
}

// Intp constructs a field that carries a *int. The returned Field will safely
// and explicitly represent `nil` when appropriate.
func Intp(key string, val *int) Field {
	if val == nil {
		return nilField(key)
	}
	return Int(key, *val)
}

// Int64 constructs a field with the given key and value.
func Int64(key string, val int64) Field {
	return zap.Int64(key, val)
}

// Int64p constructs a field that carries a *int64. The returned Field will safely
// and explicitly represent `nil` when appropriate.
func Int64p(key string, val *int64) Field {
	if val == nil {
		return nilField(key)
	}
	return Int64(key, *val)
}

// Int32 constructs a field with the given key and value.
func Int32(key string, val int32) Field {
	return zap.Int32(key, val)
}

// Int32p constructs a field that carries a *int32. The returned Field will safely
// and explicitly represent `nil` when appropriate.
func Int32p(key string, val *int32) Field {
	if val == nil {
		return nilField(key)
	}
	return Int32(key, *val)
}

// Int16 constructs a field with the given key and value.
func Int16(key string, val int16) Field {
	return zap.Int16(key, val)
}

// Int16p constructs a field that carries a *int16. The returned Field will safely
// and explicitly represent `nil` when appropriate.
func Int16p(key string, val *int16) Field {
	if val == nil {
		return nilField(key)
	}
	return Int16(key, *val)
}

// Int8 constructs a field with the given key and value.
func Int8(key string, val int8) Field {
	return zap.Int8(key, val)
}

// Int8p constructs a field that carries a *int8. The returned Field will safely
// and explicitly represent `nil` when appropriate.
func Int8p(key string, val *int8) Field {
	if val == nil {
		return nilField(key)
	}
	return Int8(key, *val)
}

// String constructs a field with the given key and value.
func String(key string, val string) Field {
	return zap.String(key, val)
}

// Stringp constructs a field that carries a *string. The returned Field will safely
// and explicitly represent `nil` when appropriate.
func Stringp(key string, val *string) Field {
	if val == nil {
		return nilField(key)
	}
	return String(key, *val)
}

// Uint constructs a field with the given key and value.
func Uint(key string, val uint) Field {
	return zap.Uint(key, val)
}

// Uintp constructs a field that carries a *uint. The returned Field will safely
// and explicitly represent `nil` when appropriate.
func Uintp(key string, val *uint) Field {
	if val == nil {
		return nilField(key)
	}
	return Uint(key, *val)
}

// Uint64 constructs a field with the given key and value.
func Uint64(key string, val uint64) Field {
	return zap.Uint64(key, val)
}

// Uint64p constructs a field that carries a *uint64. The returned Field will safely
// and explicitly represent `nil` when appropriate.
func Uint64p(key string, val *uint64) Field {
	if val == nil {
		return nilField(key)
	}
	return Uint64(key, *val)
}

// Uint32 constructs a field with the given key and value.
func Uint32(key string, val uint32) Field {
	return zap.Uint32(key, val)
}

// Uint32p constructs a field that carries a *uint32. The returned Field will safely
// and explicitly represent `nil` when appropriate.
func Uint32p(key string, val *uint32) Field {
	if val == nil {
		return nilField(key)
	}
	return Uint32(key, *val)
}

// Uint16 constructs a field with the given key and value.
func Uint16(key string, val uint16) Field {
	return zap.Uint16(key, val)
}

// Uint16p constructs a field that carries a *uint16. The returned Field will safely
// and explicitly represent `nil` when appropriate.
func Uint16p(key string, val *uint16) Field {
	if val == nil {
		return nilField(key)
	}
	return Uint16(key, *val)
}

// Uint8 constructs a field with the given key and value.
func Uint8(key string, val uint8) Field {
	return zap.Uint8(key, val)
}

// Uint8p constructs a field that carries a *uint8. The returned Field will safely
// and explicitly represent `nil` when appropriate.
func Uint8p(key string, val *uint8) Field {
	if val == nil {
		return nilField(key)
	}
	return Uint8(key, *val)
}

// Uintptr constructs a field with the given key and value.
func Uintptr(key string, val uintptr) Field {
	return zap.Uintptr(key, val)
}

// Uintptrp constructs a field that carries a *uintptr. The returned Field will safely
// and explicitly represent `nil` when appropriate.
func Uintptrp(key string, val *uintptr) Field {
	if val == nil {
		return nilField(key)
	}
	return Uintptr(key, *val)
}

// Reflect constructs a field with the given key and an arbitrary object. It uses
// an encoding-appropriate, reflection-based function to lazily serialize nearly
// any object into the logging context, but it's relatively slow and
// allocation-heavy. Outside tests, Any is always a better choice.
//
// If encoding fails (e.g., trying to serialize a map[int]string to JSON), Reflect
// includes the error message in the final log output.
func Reflect(key string, val interface{}) Field {
	return Field{Key: key, Type: zapcore.ReflectType, Interface: val}
}

// Namespace creates a named, isolated scope within the logger's context. All
// subsequent fields will be added to the create namespace.
//
// This helps prevent key collisions when injecting loggers into sub-components
// or third-party libraries.
func Namespace(key string) Field {
	return Field{Key: key, Type: zapcore.NamespaceType}
}

// Stringer constructs a field with the given key and the output of the value's
// String method. The Stringer's String method is called lazily.
func Stringer(key string, val fmt.Stringer) Field {
	return zap.Stringer(key, val)
}

// Time constructs a Field with the given key and value. The encoder
// controls how the time is serialized.
func Time(key string, val time.Time) Field {
	return zap.Time(key, val)
}

// Timep constructs a field that carries a *time.Time. The returned Field will safely
// and explicitly represent `nil` when appropriate.
func Timep(key string, val *time.Time) Field {
	if val == nil {
		return nilField(key)
	}
	return Time(key, *val)
}

// Stack constructs a field that stores a stacktrace of the current goroutine
// under provided key. Keep in mind that taking a stacktrace is eager and
// expensive (relatively speaking); this function both makes an allocation and
// takes about two microseconds.
func Stack(key string) Field {
	return StackSkip(key, 2) // skip Stack
}

// StackSkip constructs a field similarly to Stack, but also skips the given
// number of frames from the top of the stacktrace.
func StackSkip(key string, skip int) Field {
	return zap.StackSkip(key, skip+1)
}

// Duration constructs a field with the given key and value. The encoder
// controls how the duration is serialized.
func Duration(key string, val time.Duration) Field {
	return zap.Duration(key, val)
}

// Durationp constructs a field that carries a *time.Duration. The returned Field will safely
// and explicitly represent `nil` when appropriate.
func Durationp(key string, val *time.Duration) Field {
	if val == nil {
		return nilField(key)
	}
	return Duration(key, *val)
}

// Object constructs a field with the given key and ObjectMarshaler. It
// provides a flexible, but still type-safe and efficient, way to add map or
// struct-like user-defined types to the logging context. The struct's
// MarshalLogObject method is called lazily.
func Object(key string, val ObjectMarshaler) Field {
	return zap.Object(key, val)
}

// NamedError instead of ErrField, use a custom key for error field
func NamedError(key string, err error) Field {
	return zap.NamedError(key, err)
}

// ErrField for Error, because Error used log
func ErrField(err error) Field {
	return zap.NamedError("error", err)
}

// Any takes a key and an arbitrary value and chooses the best way to represent
// them as a field, falling back to a reflection-based approach only if
// necessary.
//
// Since byte/uint8 and rune/int32 are aliases, Any can't differentiate between
// them. To minimize surprises, []byte values are treated as binary blobs, byte
// values are treated as uint8, and runes are always treated as integers.
func Any(key string, val interface{}) Field {
	return zap.Any(key, val)
}

// Array constructs a field with the given key and ArrayMarshaler. It provides
// a flexible, but still type-safe and efficient, way to add array-like types
// to the logging context. The struct's MarshalLogArray method is called lazily.
func Array(key string, val ArrayMarshaler) Field {
	return zap.Array(key, val)
}

// ByteStrings constructs a field that carries a slice of []byte, each of which
// must be UTF-8 encoded text.
func ByteStrings(key string, bss [][]byte) Field {
	return zap.ByteStrings(key, bss)
}

// Complex128s constructs a field that carries a slice of complex numbers.
func Complex128s(key string, nums []complex128) Field {
	return zap.Complex128s(key, nums)
}

// Complex64s constructs a field that carries a slice of complex numbers.
func Complex64s(key string, nums []complex64) Field {
	return zap.Complex64s(key, nums)
}

// Durations constructs a field that carries a slice of time.Durations.
func Durations(key string, ds []time.Duration) Field {
	return zap.Durations(key, ds)
}

// Float64s constructs a field that carries a slice of floats.
func Float64s(key string, nums []float64) Field {
	return zap.Float64s(key, nums)
}

// Float32s constructs a field that carries a slice of floats.
func Float32s(key string, nums []float32) Field {
	return zap.Float32s(key, nums)
}

// Ints constructs a field that carries a slice of integers.
func Ints(key string, nums []int) Field {
	return zap.Ints(key, nums)
}

// Int64s constructs a field that carries a slice of integers.
func Int64s(key string, nums []int64) Field {
	return zap.Int64s(key, nums)
}

// Int32s constructs a field that carries a slice of integers.
func Int32s(key string, nums []int32) Field {
	return zap.Int32s(key, nums)
}

// Int16s constructs a field that carries a slice of integers.
func Int16s(key string, nums []int16) Field {
	return zap.Int16s(key, nums)
}

// Int8s constructs a field that carries a slice of integers.
func Int8s(key string, nums []int8) Field {
	return zap.Int8s(key, nums)
}

// Strings constructs a field that carries a slice of strings.
func Strings(key string, ss []string) Field {
	return zap.Strings(key, ss)
}

// Times constructs a field that carries a slice of time.Times.
func Times(key string, ts []time.Time) Field {
	return zap.Times(key, ts)
}

// Uints constructs a field that carries a slice of unsigned integers.
func Uints(key string, nums []uint) Field {
	return zap.Uints(key, nums)
}

// Uint64s constructs a field that carries a slice of unsigned integers.
func Uint64s(key string, nums []uint64) Field {
	return zap.Uint64s(key, nums)
}

// Uint32s constructs a field that carries a slice of unsigned integers.
func Uint32s(key string, nums []uint32) Field {
	return zap.Uint32s(key, nums)
}

// Uint16s constructs a field that carries a slice of unsigned integers.
func Uint16s(key string, nums []uint16) Field {
	return zap.Uint16s(key, nums)
}

// Uint8s constructs a field that carries a slice of unsigned integers.
func Uint8s(key string, nums []uint8) Field {
	return zap.Uint8s(key, nums)
}

// Uintptrs constructs a field that carries a slice of pointer addresses.
func Uintptrs(key string, us []uintptr) Field {
	return zap.Uintptrs(key, us)
}

// Errors constructs a field that carries a slice of errors.
func Errors(key string, errs []error) Field {
	return zap.Errors(key, errs)
}

type (
	// ObjectMarshaler alias to zapcore.ObjectMarshaler
	ObjectMarshaler = zapcore.ObjectMarshaler
	// ArrayMarshaler alias to zapcore.ArrayMarshaler
	ArrayMarshaler = zapcore.ArrayMarshaler
)
