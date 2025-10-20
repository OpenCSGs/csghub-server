package handler

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"

	"opencsg.com/csghub-server/common/types"
)

type langflowEvent struct {
	Event types.AgentStreamEvent
	Raw   []byte
}

type langflowEventStreamDecoder struct {
	buf bytes.Buffer
	dec *json.Decoder
}

// Write ingests bytes and returns all fully parsed events.
// It supports concatenated JSON objects like `{...}{...}{...}`.
func (d *langflowEventStreamDecoder) Write(p []byte) ([]*langflowEvent, error) {
	if len(p) == 0 {
		return nil, nil
	}

	// Append data to buffer
	if _, err := d.buf.Write(p); err != nil {
		return nil, err
	}

	// Recreate decoder from current buffer
	data := d.buf.Bytes()
	d.dec = json.NewDecoder(bytes.NewReader(data))

	var events []*langflowEvent
	var lastPos int64

	for {
		// Attempt to decode next JSON object
		var evt types.AgentStreamEvent
		startOffset := d.dec.InputOffset()
		if err := d.dec.Decode(&evt); err != nil {
			if err == io.EOF {
				break // wait for more data
			}
			// Not enough data? Keep remaining bytes
			if err.Error() == "unexpected end of JSON input" {
				break
			}
			slog.Error("LangflowEventStreamDecoder decode error", slog.Any("err", err))
			break
		}

		endOffset := d.dec.InputOffset()
		raw := data[startOffset:endOffset]
		events = append(events, &langflowEvent{
			Event: evt,
			Raw:   append([]byte(nil), raw...), // detach
		})
		lastPos = endOffset
	}

	// Remove parsed bytes from buffer (keep incomplete part)
	if lastPos > 0 && int(lastPos) < len(data) {
		remaining := data[lastPos:]
		d.buf.Reset()
		d.buf.Write(remaining)
	} else if lastPos > 0 {
		d.buf.Reset()
	}

	if len(events) == 0 {
		return nil, nil
	}
	return events, nil
}
