package handler

import (
	"encoding/json"
	"testing"

	"opencsg.com/csghub-server/common/types"
)

func TestLangflowEventStreamDecoder_Write(t *testing.T) {
	tests := []struct {
		name     string
		input    []byte
		expected []*langflowEvent
		wantErr  bool
	}{
		{
			name:     "empty input",
			input:    []byte{},
			expected: nil,
			wantErr:  false,
		},
		{
			name:  "single complete JSON event",
			input: []byte(`{"event":"test","data":"hello"}`),
			expected: []*langflowEvent{
				{
					Event: types.AgentStreamEvent{
						Event: "test",
						Data:  json.RawMessage(`"hello"`),
					},
					Raw: []byte(`{"event":"test","data":"hello"}`),
				},
			},
			wantErr: false,
		},
		{
			name:  "multiple complete JSON events",
			input: []byte(`{"event":"start","data":"begin"}{"event":"end","data":"finish"}`),
			expected: []*langflowEvent{
				{
					Event: types.AgentStreamEvent{
						Event: "start",
						Data:  json.RawMessage(`"begin"`),
					},
					Raw: []byte(`{"event":"start","data":"begin"}`),
				},
				{
					Event: types.AgentStreamEvent{
						Event: "end",
						Data:  json.RawMessage(`"finish"`),
					},
					Raw: []byte(`{"event":"end","data":"finish"}`),
				},
			},
			wantErr: false,
		},
		{
			name:     "incomplete JSON event",
			input:    []byte(`{"event":"test","data":"incomplete`),
			expected: nil,
			wantErr:  false,
		},
		{
			name:  "complete event followed by incomplete",
			input: []byte(`{"event":"complete","data":"done"}{"event":"incomplete","data":"not`),
			expected: []*langflowEvent{
				{
					Event: types.AgentStreamEvent{
						Event: "complete",
						Data:  json.RawMessage(`"done"`),
					},
					Raw: []byte(`{"event":"complete","data":"done"}`),
				},
			},
			wantErr: false,
		},
		{
			name:  "event with complex data",
			input: []byte(`{"event":"message","data":{"text":"Hello World","timestamp":"2023-01-01T00:00:00Z"}}`),
			expected: []*langflowEvent{
				{
					Event: types.AgentStreamEvent{
						Event: "message",
						Data:  json.RawMessage(`{"text":"Hello World","timestamp":"2023-01-01T00:00:00Z"}`),
					},
					Raw: []byte(`{"event":"message","data":{"text":"Hello World","timestamp":"2023-01-01T00:00:00Z"}}`),
				},
			},
			wantErr: false,
		},
		{
			name:  "event with null data",
			input: []byte(`{"event":"empty","data":null}`),
			expected: []*langflowEvent{
				{
					Event: types.AgentStreamEvent{
						Event: "empty",
						Data:  json.RawMessage(`null`),
					},
					Raw: []byte(`{"event":"empty","data":null}`),
				},
			},
			wantErr: false,
		},
		{
			name:  "event with array data",
			input: []byte(`{"event":"list","data":["item1","item2","item3"]}`),
			expected: []*langflowEvent{
				{
					Event: types.AgentStreamEvent{
						Event: "list",
						Data:  json.RawMessage(`["item1","item2","item3"]`),
					},
					Raw: []byte(`{"event":"list","data":["item1","item2","item3"]}`),
				},
			},
			wantErr: false,
		},
		{
			name:     "malformed JSON",
			input:    []byte(`{"event":"test","data":"hello"invalid`),
			expected: nil,
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			decoder := &langflowEventStreamDecoder{}
			events, err := decoder.Write(tt.input)

			if (err != nil) != tt.wantErr {
				t.Errorf("Write() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if len(events) != len(tt.expected) {
				t.Errorf("Write() returned %d events, expected %d", len(events), len(tt.expected))
				return
			}

			for i, event := range events {
				if event.Event.Event != tt.expected[i].Event.Event {
					t.Errorf("Event[%d].Event = %v, expected %v", i, event.Event.Event, tt.expected[i].Event.Event)
				}

				if string(event.Event.Data) != string(tt.expected[i].Event.Data) {
					t.Errorf("Event[%d].Data = %v, expected %v", i, string(event.Event.Data), string(tt.expected[i].Event.Data))
				}

				if string(event.Raw) != string(tt.expected[i].Raw) {
					t.Errorf("Event[%d].Raw = %v, expected %v", i, string(event.Raw), string(tt.expected[i].Raw))
				}
			}
		})
	}
}

func TestLangflowEventStreamDecoder_Write_ChunkedInput(t *testing.T) {
	decoder := &langflowEventStreamDecoder{}

	// First chunk - incomplete
	chunk1 := []byte(`{"event":"start","data":"begin"}{"event":"middle","data":"incomplete`)
	events, err := decoder.Write(chunk1)
	if err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("Expected 1 event, got %d", len(events))
	}
	if events[0].Event.Event != "start" {
		t.Errorf("Expected event 'start', got '%s'", events[0].Event.Event)
	}

	// Second chunk - completes the middle event and adds a new one
	chunk2 := []byte(`complete"}{"event":"end","data":"finish"}`)
	events, err = decoder.Write(chunk2)
	if err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("Expected 2 events, got %d", len(events))
	}
	if events[0].Event.Event != "middle" {
		t.Errorf("Expected first event 'middle', got '%s'", events[0].Event.Event)
	}
	if events[1].Event.Event != "end" {
		t.Errorf("Expected second event 'end', got '%s'", events[1].Event.Event)
	}
}

func TestLangflowEventStreamDecoder_Write_MultipleWrites(t *testing.T) {
	decoder := &langflowEventStreamDecoder{}

	// First write - complete event
	events, err := decoder.Write([]byte(`{"event":"first","data":"complete"}`))
	if err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("Expected 1 event, got %d", len(events))
	}

	// Second write - complete event
	events, err = decoder.Write([]byte(`{"event":"second","data":"complete"}`))
	if err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("Expected 1 event, got %d", len(events))
	}
	if events[0].Event.Event != "second" {
		t.Errorf("Expected event 'second', got '%s'", events[0].Event.Event)
	}
}

func TestLangflowEventStreamDecoder_Write_EmptyData(t *testing.T) {
	decoder := &langflowEventStreamDecoder{}

	// Test with empty data field
	events, err := decoder.Write([]byte(`{"event":"test","data":""}`))
	if err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("Expected 1 event, got %d", len(events))
	}
	if string(events[0].Event.Data) != `""` {
		t.Errorf("Expected empty string data, got %s", string(events[0].Event.Data))
	}
}

func TestLangflowEventStreamDecoder_Write_WhitespaceHandling(t *testing.T) {
	decoder := &langflowEventStreamDecoder{}

	// Test with whitespace between JSON objects
	events, err := decoder.Write([]byte(`{"event":"first","data":"1"} {"event":"second","data":"2"}`))
	if err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("Expected 2 events, got %d", len(events))
	}
	if events[0].Event.Event != "first" {
		t.Errorf("Expected first event 'first', got '%s'", events[0].Event.Event)
	}
	if events[1].Event.Event != "second" {
		t.Errorf("Expected second event 'second', got '%s'", events[1].Event.Event)
	}
}

func TestLangflowEventStreamDecoder_Write_BufferManagement(t *testing.T) {
	decoder := &langflowEventStreamDecoder{}

	// Write incomplete data
	events, err := decoder.Write([]byte(`{"event":"incomplete","data":"not`))
	if err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	if len(events) != 0 {
		t.Fatalf("Expected 0 events, got %d", len(events))
	}

	// Complete the data
	events, err = decoder.Write([]byte(`complete"}`))
	if err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("Expected 1 event, got %d", len(events))
	}
	if events[0].Event.Event != "incomplete" {
		t.Errorf("Expected event 'incomplete', got '%s'", events[0].Event.Event)
	}
	if string(events[0].Event.Data) != `"notcomplete"` {
		t.Errorf("Expected data 'notcomplete', got %s", string(events[0].Event.Data))
	}
}

func TestLangflowEventStreamDecoder_Write_RealWorldExample(t *testing.T) {
	decoder := &langflowEventStreamDecoder{}

	// Simulate a real langflow stream with multiple events
	streamData := `{"event":"start","data":{"message":"Starting process"}}{"event":"progress","data":{"percentage":50}}{"event":"token","data":{"chunk":"Hello"}}{"event":"token","data":{"chunk":" World"}}{"event":"end","data":{"result":"Hello World"}}`

	events, err := decoder.Write([]byte(streamData))
	if err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	if len(events) != 5 {
		t.Fatalf("Expected 5 events, got %d", len(events))
	}

	expectedEvents := []string{"start", "progress", "token", "token", "end"}
	for i, event := range events {
		if event.Event.Event != expectedEvents[i] {
			t.Errorf("Event[%d] = %s, expected %s", i, event.Event.Event, expectedEvents[i])
		}
	}
}
