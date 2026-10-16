package main

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// The landing ships static metadata: detect SDK changes before it can publish
// stale support or transport claims.
func TestGeneratedCapabilitiesAreCurrent(t *testing.T) {
	want, err := generate()
	if err != nil {
		t.Fatal(err)
	}
	_, source, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("locate generator source")
	}
	path := filepath.Join(filepath.Dir(source), "../../../../landing/data/clientCapabilities.generated.json")
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Fatal("landing client capabilities are stale; run from cli: go run ./internal/tools/genclientcapabilities")
	}
}
