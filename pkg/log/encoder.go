package log

import (
	"encoding/json"
	"fmt"

	"go.uber.org/zap"
	"go.uber.org/zap/buffer"
	"go.uber.org/zap/zapcore"
)

var nullLiteralBytes = []byte("null")

type wrappedJSONEncoder struct {
	zapcore.Encoder
}

func (w *wrappedJSONEncoder) encodeReflectedValue(value interface{}) ([]byte, error) {
	if value == nil {
		return nullLiteralBytes, nil
	}

	return json.Marshal(value)
}

func (w *wrappedJSONEncoder) EncodeEntry(entry zapcore.Entry, fields []zapcore.Field) (*buffer.Buffer, error) {
	// custom fields altering
	for i, f := range fields {
		switch f.Type {
		case zapcore.ReflectType:
			// reflect type will be forced encoded to ByteString to avoid es issue
			// this will alter the field name with suffix "_raw"
			b, err := w.encodeReflectedValue(f.Interface)
			if err != nil {
				return nil, fmt.Errorf("encodeReflectedValue: %w", err)
			}

			fields[i] = zap.ByteString(f.Key+"|raw", b)
		case zapcore.ArrayMarshalerType:
			fields[i].Key = f.Key + "|array"
		case zapcore.ObjectMarshalerType:
			fields[i].Key = f.Key + "|object"
		}
	}

	return w.Encoder.EncodeEntry(entry, fields)
}

func (w *wrappedJSONEncoder) Clone() zapcore.Encoder {
	return &wrappedJSONEncoder{
		Encoder: w.Encoder.Clone(),
	}
}
