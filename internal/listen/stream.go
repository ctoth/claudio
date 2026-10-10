package listen

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"log/slog"
)

// ReadEvents plays the events on r, one JSON event per line, until r ends.
// It is the listener for anything that can be piped: what carries the
// events here is not its business. Blank lines are skipped; a line that is
// not an event, or is longer than an event may be, is logged and skipped.
// The error is r's read error; a stream that simply ends returns nil.
func (s *Server) ReadEvents(r io.Reader) error {
	reader := bufio.NewReaderSize(r, maxEventBytes)
	for {
		line, tooLong, err := readLine(reader)
		switch {
		case tooLong:
			slog.Warn("event refused: line too long", "from", "stream", "limit_bytes", maxEventBytes)
		case len(bytes.TrimSpace(line)) > 0:
			s.submit(line, "stream")
		}
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("read events: %w", err)
		}
	}
}

// readLine returns the next line without its ending. A line that does not
// fit the reader's buffer is consumed whole and reported as tooLong. The
// last line of a stream comes back together with io.EOF.
func readLine(reader *bufio.Reader) (line []byte, tooLong bool, err error) {
	line, err = reader.ReadSlice('\n')
	for errors.Is(err, bufio.ErrBufferFull) {
		tooLong = true
		_, err = reader.ReadSlice('\n')
	}
	if tooLong {
		return nil, true, err
	}
	// The slice is only valid until the next read; submit decodes it first.
	return line, false, err
}
