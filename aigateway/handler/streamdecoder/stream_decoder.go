package streamdecoder

import (
	"bytes"
	"errors"
)

type Event struct {
	Type string
	Data []byte
	Raw  []byte
}

type Format string

const (
	FormatSSE    Format = "sse"
	FormatNDJSON Format = "ndjson"
)

type Decoder interface {
	Format() Format
	Write(p []byte) ([]*Event, error)
}

func NewSSE() Decoder {
	return &eventStreamDecoder{}
}

func NewNDJSON() Decoder {
	return &ndjsonStreamDecoder{}
}

type eventStreamDecoder struct {
	buf bytes.Buffer
}

func (d *eventStreamDecoder) Format() Format {
	return FormatSSE
}

func normalizeSSEBytes(data []byte) []byte {
	return bytes.ReplaceAll(data, []byte("\r\n"), []byte("\n"))
}

func (d *eventStreamDecoder) Write(p []byte) ([]*Event, error) {
	d.buf.Write(p)

	data := normalizeSSEBytes(d.buf.Bytes())
	if len(data) == 0 {
		return nil, nil
	}

	var events []*Event
	// Process all complete SSE events and keep a partial trailing event buffered.
	for {
		idx := bytes.Index(data, []byte("\n\n"))
		if idx < 0 {
			break
		}
		event := d.parseEvent(data[:idx+2])
		events = append(events, event)
		data = data[idx+2:]
	}

	// Retain incomplete data so the next Write call can finish the event.
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
	// Drop the SSE event terminator before parsing individual fields.
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

type ndjsonStreamDecoder struct {
	buf bytes.Buffer
}

func (d *ndjsonStreamDecoder) Format() Format {
	return FormatNDJSON
}

const maxNDJSONLineSize = 16 << 20 // 16 MiB

var ErrNDJSONLineTooLarge = errors.New("ndjson line too large")

func (d *ndjsonStreamDecoder) Write(p []byte) ([]*Event, error) {
	if _, err := d.buf.Write(p); err != nil {
		return nil, err
	}
	b := d.buf.Bytes()
	events := make([]*Event, 0)
	start := 0
	for i := 0; i < len(b); i++ {
		if b[i] != '\n' {
			continue
		}
		line := b[start:i]
		if len(line) > 0 && line[len(line)-1] == '\r' {
			line = line[:len(line)-1]
		}
		if len(line) > maxNDJSONLineSize {
			return nil, ErrNDJSONLineTooLarge
		}
		if len(line) > 0 {
			raw := bytes.Clone(b[start : i+1])
			data := bytes.Clone(line)
			events = append(events, &Event{
				Data: data,
				Raw:  raw,
			})
		}
		start = i + 1
	}
	if len(b[start:]) > maxNDJSONLineSize {
		return nil, ErrNDJSONLineTooLarge
	}
	if start > 0 {
		remaining := bytes.Clone(b[start:])
		d.buf.Reset()
		if len(remaining) > 0 {
			_, _ = d.buf.Write(remaining)
		}
	}
	return events, nil
}
