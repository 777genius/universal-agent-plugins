package vscodelocalhooks_test

import (
	"crypto/sha256"
	"encoding/json"
	"os"
	"runtime"
	"testing"
)

// Red: required native qualification silently runs on another host or skips.
// CI sets the exact OS; this all-host test also survives native file removal.
func TestRequiredNativeHost(t *testing.T) { requireNativeHost(t) }

func requireNativeHost(t *testing.T) {
	t.Helper()
	if expected := os.Getenv("U2_REQUIRE_NATIVE"); expected != "" && (expected != runtime.GOOS || (expected != "windows" && expected != "darwin")) {
		t.Fatalf("required native host %q unavailable on %s", expected, runtime.GOOS)
	}
}

func copyTestExecutable(t *testing.T, destination string) {
	t.Helper()
	source, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(source)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(destination, body, 0700); err != nil {
		t.Fatal(err)
	}
	t.Logf("TEST inert recorder SHA-256: %x", sha256.Sum256(body))
}

func nativePlatformCommand(t *testing.T, body []byte, field string) string {
	t.Helper()
	var native map[string]map[string][]map[string]any
	if err := json.Unmarshal(body, &native); err != nil {
		t.Fatal(err)
	}
	entries := native["hooks"]["Stop"]
	if len(entries) != 1 {
		t.Fatalf("missing native entry: %s", body)
	}
	command, ok := entries[0][field].(string)
	if !ok || command == "" || len(entries[0]) != 3 {
		t.Fatalf("incorrect native platform: %s", body)
	}
	return command
}
