package checksum

import (
	"bufio"
	"bytes"
	"encoding/hex"
	"fmt"
	"strings"
)

// ExpectedSum parses checksums.txt (GoReleaser / shasum style) and returns the SHA256
// hex string for the given archive file name, or error if missing or invalid.
func ExpectedSum(checksumsFile []byte, archiveName string) ([]byte, error) {
	sc := bufio.NewScanner(bytes.NewReader(checksumsFile))
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) < 2 {
			continue
		}
		hashHex := parts[0]
		// Filename may be last field (handles "hash  file" and "hash *file")
		fileName := strings.TrimPrefix(parts[len(parts)-1], "*")
		if !strings.EqualFold(fileName, archiveName) {
			continue
		}
		if len(hashHex) != 64 {
			return nil, fmt.Errorf("invalid hash length for %s", archiveName)
		}
		sum, err := hex.DecodeString(hashHex)
		if err != nil {
			return nil, fmt.Errorf("decode hash: %w", err)
		}
		return sum, nil
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	return nil, fmt.Errorf("no checksum line for %q", archiveName)
}
