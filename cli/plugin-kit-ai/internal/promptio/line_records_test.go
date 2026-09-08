package promptio

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli/prompt"
)

// Model canonical read boundaries, including nonempty VEOF records. This tests
// the parser on Linux; the Darwin PTY tests exercise the actual kernel contract.
type canonicalRecords struct{ records []string }

func (r *canonicalRecords) Read(p []byte) (int, error) {
	if len(r.records) == 0 {
		return 0, io.EOF
	}
	record := r.records[0]
	n := copy(p, record)
	if n == len(record) {
		r.records = r.records[1:]
	} else {
		r.records[0] = record[n:]
	}
	if n == 0 {
		return 0, io.EOF
	}
	return n, nil
}

func TestCanonicalRecordsPreserveNextOwner(t *testing.T) {
	for _, records := range [][]string{
		{"y\n", "next\n"},
		{"y", "\n", "next\n"},
		{strings.Repeat("x", 4096) + "\n", "next\n"},
	} {
		r := &canonicalRecords{records: append([]string(nil), records...)}
		want := strings.TrimSuffix(strings.Join(records[:len(records)-1], ""), "\n")
		line, err := readLineBuffer(context.Background(), r, make([]byte, 4097))
		if line != want || err != nil {
			t.Fatalf("answer length %d: %v", len(line), err)
		}
		if len(r.records) != 1 || r.records[0] != "next\n" {
			t.Fatalf("prefetched next owner: %q", r.records)
		}
	}
}

func TestCanonicalRecordsRejectPartialEOFAndOverlong(t *testing.T) {
	r := &canonicalRecords{records: []string{"y", "", "next\n"}}
	if line, err := readLineBuffer(context.Background(), r, make([]byte, 4097)); line != "" || !errors.Is(err, prompt.ErrPromptInputClosed) {
		t.Fatalf("partial consent: %q %v", line, err)
	}
	if len(r.records) != 1 || r.records[0] != "next\n" {
		t.Fatalf("next owner changed: %q", r.records)
	}
	r = &canonicalRecords{records: []string{strings.Repeat("x", 2048), strings.Repeat("x", 2049), "next\n"}}
	if line, err := readLineBuffer(context.Background(), r, make([]byte, 4097)); line != "" || err == nil || !strings.Contains(err.Error(), "exceeds 4096") {
		t.Fatalf("overlong answer: %q %v", line, err)
	}
	if len(r.records) != 1 || r.records[0] != "next\n" {
		t.Fatalf("next owner changed: %q", r.records)
	}
}
