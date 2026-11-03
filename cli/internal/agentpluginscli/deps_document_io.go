package agentpluginscli

import (
	"fmt"
	"path/filepath"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/atomicfile"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/dependencymanifest"
)

const (
	// These are authority documents, not arbitrary package archives. Bound both
	// on-disk reads and in-memory decodes before allocating/parsing hostile input.
	maxDependencyManifestBytes int64 = dependencymanifest.MaxBytes
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

// validateDependencyCandidateFiles admits both replacements together before
// any manifest-first publication; a later oversized lock cannot leave a
// partially updated agreement from this operation.
func validateDependencyCandidateFiles(manifest, lock []byte) error {
	if int64(len(manifest)) > maxDependencyManifestBytes {
		return fmt.Errorf("uap.toml exceeds %d bytes", maxDependencyManifestBytes)
	}
	if int64(len(lock)) > maxDependencyLockBytes {
		return fmt.Errorf("uap.lock exceeds %d bytes", maxDependencyLockBytes)
	}
	return nil
}

func readDependencyDocument(path string, limit int64) ([]byte, error) {
	return atomicfile.ReadBoundedRegularFile(path, limit)
}
