package native

import (
	"io"
)

// zeroReader returns zero bytes until remaining runs out. Used to exceed the
// size cap without holding the data in the test.
type zeroReader struct {
	remaining int64
}

func (z *zeroReader) Read(p []byte) (int, error) {
	if z.remaining <= 0 {
		return 0, io.EOF
	}
	n := min(int64(len(p)), z.remaining)
	clear(p[:n])
	z.remaining -= n
	return int(n), nil
}
