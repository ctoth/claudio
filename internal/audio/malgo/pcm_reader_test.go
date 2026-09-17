//go:build cgo

package malgo

import (
	"bytes"
	"context"
	"errors"
	"io"
	"testing"
)

type finalPCMReader struct{ read bool }

func (r *finalPCMReader) Read(p []byte) (int, error) {
	if r.read {
		return 0, io.EOF
	}
	r.read = true
	return copy(p, []byte{1, 2, 3, 4}), io.EOF
}

func TestReadDecodedPCMPreservesFinalBytesAndBoundsExpansion(t *testing.T) {
	pcm, err := readDecodedPCM(context.Background(), &finalPCMReader{}, 4)
	if err != nil || !bytes.Equal(pcm, []byte{1, 2, 3, 4}) {
		t.Fatalf("final bytes lost: %v, %v", pcm, err)
	}
	source := bytes.NewReader(make([]byte, 100))
	if _, err := readDecodedPCM(context.Background(), source, 4); err == nil {
		t.Fatal("expanded PCM exceeded limit")
	}
	if source.Len() != 95 {
		t.Fatalf("read beyond limit probe: %d bytes remain", source.Len())
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := readDecodedPCM(ctx, bytes.NewReader([]byte{1}), 4); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation lost: %v", err)
	}
}
