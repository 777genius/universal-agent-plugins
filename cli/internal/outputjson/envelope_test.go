package outputjson

import (
	"bytes"
	"errors"
	"testing"
)

func TestEnvelopeFrozenBytes(t *testing.T) {
	for _, result := range []string{Success, Failure} {
		var b bytes.Buffer
		err := Write(&b, "author.inspect", result, map[string]any{"value": "<example>&"})
		expected := "{\"schema_version\":1,\"command\":\"author.inspect\",\"result\":\"" + result + "\",\"data\":{\"value\":\"<example>&\"}}\n"
		if err != nil || b.String() != expected {
			t.Fatalf("bytes %q, error %v", b.String(), err)
		}
	}
}

type brokenWriter struct{ calls int }

var writeFailure = errors.New("output unavailable")

func (w *brokenWriter) Write([]byte) (int, error) { w.calls++; return 0, writeFailure }
func TestEnvelopeWritesOnceAndPropagatesFailure(t *testing.T) {
	w := &brokenWriter{}
	if err := Write(w, "author", Failure, struct{}{}); !errors.Is(err, writeFailure) || w.calls != 1 {
		t.Fatalf("error %v calls %d", err, w.calls)
	}
	var b bytes.Buffer
	if err := Write(&b, "author", Success, make(chan int)); err == nil || b.Len() != 0 {
		t.Fatalf("encoding failure wrote %q: %v", b.String(), err)
	}
}
