package deploy

import (
	"bufio"
	"errors"
	"io"
	"log/slog"
)

type MultiLogReader struct {
	buildReader  io.ReadCloser
	runnerReader io.ReadCloser
}

func (r *MultiLogReader) Close() error {
	var err error
	if r.buildReader != nil {
		err = r.buildReader.Close()
	}

	if r.runnerReader != nil {
		err = errors.Join(err, r.runnerReader.Close())
	}
	return err
}

func (r *MultiLogReader) BuildLog() <-chan []byte {
	output := make(chan []byte)
	go r.readToChannel(r.buildReader, output)
	return output
}

func (r *MultiLogReader) RunLog() <-chan []byte {
	output := make(chan []byte, 4)
	go r.readToChannel(r.runnerReader, output)
	return output
}

func (r *MultiLogReader) readToChannel(rc io.ReadCloser, output chan []byte) {
	buf := make([]byte, 256)
	br := bufio.NewReader(rc)

	for {
		n, err := br.Read(buf)
		if err != nil {
			slog.Error("multi log reader get EOF from inner log reader", slog.Any("error", err))
			rc.Close()
			close(output)
			break
		}

		if n > 0 {
			data := make([]byte, n)
			copy(data, buf)
			output <- data
		}
	}
}
