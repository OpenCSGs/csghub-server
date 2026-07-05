package streamdecoder

import (
	"bytes"
	"errors"
	"reflect"
	"testing"
)

func TestEventStreamDecoder_Write(t *testing.T) {
	tests := []struct {
		name     string
		inputs   [][]byte
		wantEvts []*Event
		wantErr  bool
	}{
		{
			name: "complete event",
			inputs: [][]byte{
				[]byte("event: test\ndata: hello world\n\n"),
			},
			wantEvts: []*Event{
				{
					Type: "test",
					Data: []byte("hello world"),
					Raw:  []byte("event: test\ndata: hello world\n\n"),
				},
			},
			wantErr: false,
		},
		{
			name: "multiple complete events",
			inputs: [][]byte{
				[]byte("event: test1\ndata: hello\n\nevent: test2\ndata: world\n\n"),
			},
			wantEvts: []*Event{
				{
					Type: "test1",
					Data: []byte("hello"),
					Raw:  []byte("event: test1\ndata: hello\n\n"),
				},
				{
					Type: "test2",
					Data: []byte("world"),
					Raw:  []byte("event: test2\ndata: world\n\n"),
				},
			},
			wantErr: false,
		},
		{
			name: "event received in chunks",
			inputs: [][]byte{
				[]byte("event: test\n"),
				[]byte("data: hello "),
				[]byte("world\n\n"),
			},
			wantEvts: []*Event{
				{
					Type: "test",
					Data: []byte("hello world"),
					Raw:  []byte("event: test\ndata: hello world\n\n"),
				},
			},
			wantErr: false,
		},
		{
			name: "event without type",
			inputs: [][]byte{
				[]byte("data: hello world\n\n"),
			},
			wantEvts: []*Event{
				{
					Type: "",
					Data: []byte("hello world"),
					Raw:  []byte("data: hello world\n\n"),
				},
			},
			wantErr: false,
		},
		{
			name: "event with multiline data",
			inputs: [][]byte{
				[]byte("event: test\ndata: line1\ndata: line2\n\n"),
			},
			wantEvts: []*Event{
				{
					Type: "test",
					Data: []byte("line1\nline2"),
					Raw:  []byte("event: test\ndata: line1\ndata: line2\n\n"),
				},
			},
			wantErr: false,
		},
		{
			name: "incomplete event",
			inputs: [][]byte{
				[]byte("event: test\ndata: hello world\n"),
			},
			wantEvts: []*Event{},
			wantErr:  false,
		},
		{
			name: "complete event with CRLF boundary",
			inputs: [][]byte{
				[]byte("event: test\r\ndata: hello world\r\n\r\n"),
			},
			wantEvts: []*Event{
				{
					Type: "test",
					Data: []byte("hello world"),
					Raw:  []byte("event: test\ndata: hello world\n\n"),
				},
			},
			wantErr: false,
		},
		{
			name: "CRLF event received in chunks",
			inputs: [][]byte{
				[]byte("event: test\r\n"),
				[]byte("data: hello "),
				[]byte("world\r\n\r\n"),
			},
			wantEvts: []*Event{
				{
					Type: "test",
					Data: []byte("hello world"),
					Raw:  []byte("event: test\ndata: hello world\n\n"),
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			decoder := &eventStreamDecoder{}
			var gotEvts []*Event

			for _, input := range tt.inputs {
				tmpEvts, err := decoder.Write(input)
				if (err != nil) != tt.wantErr {
					t.Errorf("Write() error = %v, wantErr %v", err, tt.wantErr)
					return
				}
				gotEvts = append(gotEvts, tmpEvts...)
			}

			var filteredGotEvts []*Event
			for _, evt := range gotEvts {
				if evt != nil {
					filteredGotEvts = append(filteredGotEvts, evt)
				}
			}

			var filteredWantEvts []*Event
			for _, evt := range tt.wantEvts {
				if evt != nil {
					filteredWantEvts = append(filteredWantEvts, evt)
				}
			}

			if !reflect.DeepEqual(filteredGotEvts, filteredWantEvts) {
				t.Errorf("Write() gotEvts = %v, want %v", filteredGotEvts, filteredWantEvts)
			}

			if tt.name == "incomplete event" {
				if len(gotEvts) != 0 {
					t.Errorf("Incomplete event should return empty slice, but got %v", gotEvts)
				}

				expectedBuf := []byte("event: test\ndata: hello world\n")
				if !bytes.Equal(decoder.buf.Bytes(), expectedBuf) {
					t.Errorf("Buffer content doesn't match expected, got = %v, want = %v",
						decoder.buf.String(), string(expectedBuf))
				}
			}
		})
	}
}

func TestNDJSONStreamDecoder_Write(t *testing.T) {
	tests := []struct {
		name     string
		inputs   [][]byte
		wantEvts []*Event
		wantBuf  []byte
	}{
		{
			name: "single line",
			inputs: [][]byte{
				[]byte(`{"text":"hello"}` + "\n"),
			},
			wantEvts: []*Event{
				{
					Data: []byte(`{"text":"hello"}`),
					Raw:  []byte(`{"text":"hello"}` + "\n"),
				},
			},
		},
		{
			name: "multiple lines",
			inputs: [][]byte{
				[]byte(`{"text":"hello"}` + "\n" + `{"text":"world"}` + "\n"),
			},
			wantEvts: []*Event{
				{
					Data: []byte(`{"text":"hello"}`),
					Raw:  []byte(`{"text":"hello"}` + "\n"),
				},
				{
					Data: []byte(`{"text":"world"}`),
					Raw:  []byte(`{"text":"world"}` + "\n"),
				},
			},
		},
		{
			name: "partial line across writes",
			inputs: [][]byte{
				[]byte(`{"text":"hel`),
				[]byte(`lo"}` + "\n"),
			},
			wantEvts: []*Event{
				{
					Data: []byte(`{"text":"hello"}`),
					Raw:  []byte(`{"text":"hello"}` + "\n"),
				},
			},
		},
		{
			name: "CRLF line ending",
			inputs: [][]byte{
				[]byte(`{"text":"hello"}` + "\r\n"),
			},
			wantEvts: []*Event{
				{
					Data: []byte(`{"text":"hello"}`),
					Raw:  []byte(`{"text":"hello"}` + "\r\n"),
				},
			},
		},
		{
			name: "empty lines ignored",
			inputs: [][]byte{
				[]byte("\n" + `{"text":"hello"}` + "\n\r\n"),
			},
			wantEvts: []*Event{
				{
					Data: []byte(`{"text":"hello"}`),
					Raw:  []byte(`{"text":"hello"}` + "\n"),
				},
			},
		},
		{
			name: "incomplete trailing data retained",
			inputs: [][]byte{
				[]byte(`{"text":"hello"}` + "\n" + `{"text":"partial`),
			},
			wantEvts: []*Event{
				{
					Data: []byte(`{"text":"hello"}`),
					Raw:  []byte(`{"text":"hello"}` + "\n"),
				},
			},
			wantBuf: []byte(`{"text":"partial`),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			decoder := &ndjsonStreamDecoder{}
			var gotEvts []*Event

			for _, input := range tt.inputs {
				tmpEvts, err := decoder.Write(input)
				if err != nil {
					t.Fatalf("Write() error = %v", err)
				}
				gotEvts = append(gotEvts, tmpEvts...)
			}

			if !reflect.DeepEqual(gotEvts, tt.wantEvts) {
				t.Errorf("Write() gotEvts = %v, want %v", gotEvts, tt.wantEvts)
			}

			if !bytes.Equal(decoder.buf.Bytes(), tt.wantBuf) {
				t.Errorf("Buffer content doesn't match expected, got = %q, want = %q",
					decoder.buf.String(), string(tt.wantBuf))
			}
		})
	}
}

func TestNDJSONStreamDecoder_LineSizeLimit(t *testing.T) {
	t.Run("incomplete line over limit", func(t *testing.T) {
		decoder := &ndjsonStreamDecoder{}

		_, err := decoder.Write(bytes.Repeat([]byte("a"), maxNDJSONLineSize+1))

		if !errors.Is(err, ErrNDJSONLineTooLarge) {
			t.Fatalf("Write() error = %v, want %v", err, ErrNDJSONLineTooLarge)
		}
	})

	t.Run("complete line over limit", func(t *testing.T) {
		decoder := &ndjsonStreamDecoder{}
		input := append(bytes.Repeat([]byte("a"), maxNDJSONLineSize+1), '\n')

		_, err := decoder.Write(input)

		if !errors.Is(err, ErrNDJSONLineTooLarge) {
			t.Fatalf("Write() error = %v, want %v", err, ErrNDJSONLineTooLarge)
		}
	})

	t.Run("many small lines over total limit", func(t *testing.T) {
		decoder := &ndjsonStreamDecoder{}
		line := append(bytes.Repeat([]byte("a"), 1024), '\n')
		lineCount := maxNDJSONLineSize/len(line) + 1
		input := bytes.Repeat(line, lineCount)

		events, err := decoder.Write(input)

		if err != nil {
			t.Fatalf("Write() error = %v", err)
		}
		if len(events) != lineCount {
			t.Fatalf("Write() got %d events, want %d", len(events), lineCount)
		}
	})
}
