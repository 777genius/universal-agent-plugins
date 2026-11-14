package process

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestReadAllLimited_RejectsOversizedPayload(t *testing.T) {
	body := bytes.Repeat([]byte("a"), 9)
	_, err := readAllLimited(context.Background(), bytes.NewReader(body), 8, "stdin payload")
	if err == nil || !strings.Contains(err.Error(), "exceeds max payload size") {
		t.Fatalf("err = %v", err)
	}
}

func TestReadAllLimited_AllowsPayloadWithinLimit(t *testing.T) {
	body := []byte("12345678")
	got, err := readAllLimited(context.Background(), bytes.NewReader(body), len(body), "stdin payload")
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(body) {
		t.Fatalf("got = %q", got)
	}
}
