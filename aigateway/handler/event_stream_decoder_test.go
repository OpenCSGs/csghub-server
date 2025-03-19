package handler

import (
	"bytes"
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			decoder := &eventStreamDecoder{}
			var gotEvts []*Event

			// Process each input chunk
			for _, input := range tt.inputs {
				tmpEvts, err := decoder.Write(input)
				if (err != nil) != tt.wantErr {
					t.Errorf("Write() error = %v, wantErr %v", err, tt.wantErr)
					return
				}
				gotEvts = append(gotEvts, tmpEvts...)
			}

			// Filter out nil events, unless the expected result is nil
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

			// Special handling for incomplete event test case
			if tt.name == "incomplete event" {
				if len(gotEvts) != 0 {
					t.Errorf("Incomplete event should return empty slice, but got %v", gotEvts)
				}

				// Check if buffer retained the incomplete data
				expectedBuf := []byte("event: test\ndata: hello world\n")
				if !bytes.Equal(decoder.buf.Bytes(), expectedBuf) {
					t.Errorf("Buffer content doesn't match expected, got = %v, want = %v",
						decoder.buf.String(), string(expectedBuf))
				}
			}
		})
	}
}
