package promptio

import (
	"context"
	"errors"
	"io"
	"os"
	"strings"
	"testing"
	"time"
)

func TestCancellationDoesNotCloseInput(t *testing.T) {
	r, w, e := os.Pipe()
	if e != nil {
		t.Fatal(e)
	}
	defer r.Close()
	defer w.Close()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { _, e := ReadLine(ctx, r); done <- e }()
	cancel()
	select {
	case e := <-done:
		if !errors.Is(e, context.Canceled) {
			t.Fatal(e)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("read leaked after cancellation")
	}
	go func() { _, _ = io.WriteString(w, "next\n") }()
	line, e := ReadLine(context.Background(), r)
	if e != nil || line != "next" {
		t.Fatal(line, e)
	}
}
func TestLineBoundaries(t *testing.T) {
	r := strings.NewReader("中文é\r\nnext\ny")
	for _, want := range []string{"中文é", "next"} {
		s, e := ReadLine(context.Background(), r)
		if s != want || e != nil {
			t.Fatal(s, e)
		}
	}
	if s, e := ReadLine(context.Background(), r); e == nil || s != "" {
		t.Fatal(s, e)
	}
}

func TestBlockedPlainReadDeadline(t *testing.T) {
	r, w, e := os.Pipe()
	if e != nil {
		t.Fatal(e)
	}
	defer r.Close()
	defer w.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	if _, e := ReadLine(ctx, r); !errors.Is(e, context.DeadlineExceeded) {
		t.Fatal(e)
	}
	go func() { _, _ = io.WriteString(w, "still open\n") }()
	s, e := ReadLine(context.Background(), r)
	if s != "still open" || e != nil {
		t.Fatal(s, e)
	}
}

// Reader may legally return its final byte and EOF in the same call.
type finalByteEOF struct{ text string }

func (r *finalByteEOF) Read(p []byte) (int, error) {
	if len(r.text) == 0 {
		return 0, io.EOF
	}
	p[0] = r.text[0]
	r.text = r.text[1:]
	if len(r.text) == 0 {
		return 1, io.EOF
	}
	return 1, nil
}
func TestFinalByteWithEOFRequiresNewline(t *testing.T) {
	for _, input := range []string{"y\n", "y\r\n", "y"} {
		line, err := ReadLine(context.Background(), &finalByteEOF{text: input})
		if strings.HasSuffix(input, "\n") {
			if line != "y" || err != nil {
				t.Fatalf("%q: %q %v", input, line, err)
			}
		} else if line != "" || err == nil {
			t.Fatalf("unterminated consent accepted: %q %v", line, err)
		}
	}
}
