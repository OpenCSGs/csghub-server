package handler

import (
	"bytes"
)

type Event struct {
	Type string
	Data []byte
	Raw  []byte
}

// var _ Decoder = (*eventStreamDecoder)(nil)

type eventStreamDecoder struct {
	buf bytes.Buffer
}

func (d *eventStreamDecoder) Write(p []byte) ([]*Event, error) {
	d.buf.Write(p)

	// Check if we have a complete event (ends with double newline)
	data := d.buf.Bytes()
	if len(data) == 0 {
		return nil, nil
	}

	var events []*Event
	// Process all complete events in the buffer
	for {
		idx := bytes.Index(data, []byte("\n\n"))
		if idx < 0 {
			break
		}
		event := d.parseEvent(data[:idx+2])
		events = append(events, event)
		data = data[idx+2:]
	}

	// Keep any remaining incomplete data in the buffer
	d.buf.Reset()
	if len(data) > 0 {
		d.buf.Write(data)
	}

	if len(events) == 0 {
		return nil, nil
	}
	return events, nil
}

func (d *eventStreamDecoder) parseEvent(data []byte) *Event {
	var event Event
	event.Raw = data
	//trim the last \n\n
	data = data[:len(data)-2]

	lines := bytes.Split(data, []byte("\n"))
	var dataLines [][]byte

	for _, line := range lines {
		if len(line) == 0 {
			continue
		}
		if bytes.HasPrefix(line, []byte("event:")) {
			event.Type = string(bytes.TrimSpace(line[6:]))
		} else if bytes.HasPrefix(line, []byte("data:")) {
			dataLines = append(dataLines, bytes.TrimSpace(line[5:]))
		}
	}

	if len(dataLines) > 0 {
		event.Data = bytes.Join(dataLines, []byte("\n"))
	}

	return &event
}
