package app

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"path/filepath"
	"strings"
)

func verifyBundleChecksum(body, expected []byte) error {
	got := sha256.Sum256(body)
	if len(expected) != len(got) || !equalBytes(got[:], expected) {
		return fmt.Errorf("sha256 mismatch")
	}
	return nil
}

func parseBundleChecksum(body []byte, wantName string) ([]byte, error) {
	lines := checksumLines(body)
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) == 1 && isHexChecksum(fields[0]) {
			return hex.DecodeString(fields[0])
		}
		if len(fields) < 2 || !isHexChecksum(fields[0]) {
			continue
		}
		name := checksumEntryName(fields)
		if wantName == "" || filepath.Base(name) == filepath.Base(wantName) || name == wantName {
			return hex.DecodeString(fields[0])
		}
	}
	if wantName != "" {
		return nil, fmt.Errorf("no checksum entry for %s", wantName)
	}
	return nil, fmt.Errorf("no checksum entry found")
}

func checksumLines(body []byte) []string {
	lines := strings.Split(strings.ReplaceAll(string(body), "\r\n", "\n"), "\n")
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		out = append(out, line)
	}
	return out
}

func checksumEntryName(fields []string) string {
	return strings.TrimPrefix(fields[len(fields)-1], "*")
}

func isHexChecksum(s string) bool {
	if len(strings.TrimSpace(s)) != 64 {
		return false
	}
	_, err := hex.DecodeString(strings.TrimSpace(s))
	return err == nil
}

func equalBytes(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
