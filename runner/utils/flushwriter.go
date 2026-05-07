package utils

import (
	"io"
	"net/http"
)

// Wrap wraps an io.Writer into a writer that flushes after every write if
// the writer implements the Flusher interface.
func Wrap(w io.Writer) io.Writer {
	fw := &flushWriter{
		writer: w,
	}
	if flusher, ok := w.(http.Flusher); ok {
		fw.flusher = flusher
	}
	return fw
}

// flushWriter provides wrapper for responseWriter with HTTP streaming capabilities
type flushWriter struct {
	flusher http.Flusher
	writer  io.Writer
}

// Write is a FlushWriter implementation of the io.Writer that sends any buffered
// data to the client.
func (fw *flushWriter) Write(p []byte) (n int, err error) {
	n, err = fw.writer.Write(p)
	if err != nil {
		return
	}
	if fw.flusher != nil {
		fw.flusher.Flush()
	}
	return
}
