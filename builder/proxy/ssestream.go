package proxy

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/tidwall/gjson"
)

// Default openai stream component
type Stream[T any] struct {
	decoder Decoder
	cur     T
	err     error
	done    bool
}

func NewStream[T any](decoder Decoder, err error) *Stream[T] {
	return &Stream[T]{
		decoder: decoder,
		err:     err,
	}
}

// Next returns false if the stream has ended or an error occurred.
// Call Stream.Current() to get the current value.
// Call Stream.Err() to get the error.
//
//		for stream.Next() {
//			data := stream.Current()
//		}
//
//	 	if stream.Err() != nil {
//			...
//	 	}
func (s *Stream[T]) Next() bool {
	if s.err != nil {
		return false
	}

	for s.decoder.Next() {
		if s.done {
			continue
		}

		if bytes.HasPrefix(s.decoder.Event().Data, []byte("[DONE]")) {
			// In this case we don't break because we still want to iterate through the full stream.
			s.done = true
			continue
		}

		if s.decoder.Event().Type == "" {
			ep := gjson.GetBytes(s.decoder.Event().Data, "error")
			if ep.Exists() {
				s.err = fmt.Errorf("received error while streaming: %s", ep.String())
				return false
			}
			s.err = json.Unmarshal(s.decoder.Event().Data, &s.cur)
			return s.err == nil
		} else {
			ep := gjson.GetBytes(s.decoder.Event().Data, "error")
			if ep.Exists() {
				s.err = fmt.Errorf("received error while streaming: %s", ep.String())
				return false
			}
			s.err = json.Unmarshal([]byte(fmt.Sprintf(`{ "event": %q, "data": %s }`, s.decoder.Event().Type, s.decoder.Event().Data)), &s.cur)
			return s.err == nil
		}
	}

	// decoder.Next() may be false because of an error
	s.err = s.decoder.Err()

	return false
}

func (s *Stream[T]) Current() T {
	return s.cur
}

func (s *Stream[T]) Err() error {
	return s.err
}

func (s *Stream[T]) Close() error {
	if s.decoder == nil {
		// already closed
		return nil
	}
	return s.decoder.Close()
}
