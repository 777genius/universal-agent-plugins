package agentpluginscli

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/atomicfile"
)

const (
	// These are authority documents, not arbitrary package archives. Bound both
	// on-disk reads and in-memory decodes before allocating/parsing hostile input.
	maxDependencyManifestBytes int64 = 4 << 20
	maxDependencyLockBytes     int64 = 32 << 20
)

// readDependencyDocument reads a stable regular file with a hard byte ceiling.
// Reject leaf symlinks and nonregular files without draining attacker-owned FIFOs.
func dependencyDocumentLimit(path string) (int64, error) {
	switch filepath.Base(path) {
	case "uap.toml":
		return maxDependencyManifestBytes, nil
	case "uap.lock":
		return maxDependencyLockBytes, nil
	default:
		return 0, fmt.Errorf("unsupported dependency agreement file")
	}
}

func readDependencyDocument(path string, limit int64) ([]byte, error) {
	before, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !before.Mode().IsRegular() {
		return nil, fmt.Errorf("%s must be a regular, non-symlink agreement file", filepath.Base(path))
	}
	if before.Size() > limit {
		return nil, fmt.Errorf("%s exceeds %d bytes", filepath.Base(path), limit)
	}
	file, err := atomicfile.OpenReadFile(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	opened, err := file.Stat()
	if err != nil {
		return nil, err
	}
	if !opened.Mode().IsRegular() || !os.SameFile(before, opened) {
		return nil, fmt.Errorf("%s changed while opening agreement file", filepath.Base(path))
	}
	body, err := io.ReadAll(io.LimitReader(file, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(body)) > limit {
		return nil, fmt.Errorf("%s exceeds %d bytes", filepath.Base(path), limit)
	}
	after, err := file.Stat()
	if err != nil {
		return nil, err
	}
	if !os.SameFile(opened, after) || opened.Size() != after.Size() || !opened.ModTime().Equal(after.ModTime()) {
		return nil, fmt.Errorf("%s changed during agreement read", filepath.Base(path))
	}
	return body, nil
}
