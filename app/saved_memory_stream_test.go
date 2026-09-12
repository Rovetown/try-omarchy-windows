package main

import (
	"bytes"
	"errors"
	"io"
	"testing"
)

func TestSavedMemoryStreamRequiresCompleteBoundedInput(t *testing.T) {
	for _, size := range []int{15, 16, 17, 32} {
		source := bytes.NewReader(bytes.Repeat([]byte{0x42}, size))
		var destination bytes.Buffer
		err := copySavedMemoryStream(&destination, source, 16)
		if size <= 16 {
			if err != nil || destination.Len() != size {
				t.Fatalf("complete stream size %d: %v", size, err)
			}
		} else if err == nil || destination.Len() > 17 {
			t.Fatalf("oversized stream accepted: %d, %v", destination.Len(), err)
		}
	}
	failure := errors.New("connection lost")
	source := io.MultiReader(bytes.NewReader([]byte("prefix")), failingSavedMemoryReader{failure})
	if err := copySavedMemoryStream(io.Discard, source, 16); !errors.Is(err, failure) {
		t.Fatalf("lost truncated-stream error: %v", err)
	}
}

type failingSavedMemoryReader struct{ err error }

func (r failingSavedMemoryReader) Read([]byte) (int, error) { return 0, r.err }
