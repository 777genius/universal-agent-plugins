//go:build windows

package agentpluginscli

import (
	"bytes"
	"io"
	"path/filepath"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/atomicfile"
)

// Read and publication must preserve Windows delete-sharing. An ordinary
// os.Open reader prevents atomic replacement on Windows and is not acceptable.
func TestDependencyAgreementWindowsHeldReaderAllowsAtomicPublication(t *testing.T) {
	path := filepath.Join(t.TempDir(), "uap.lock")
	oldBytes := []byte("old agreement bytes")
	newBytes := []byte("new agreement bytes")
	if err := atomicfile.Write(path, oldBytes, 0o600); err != nil {
		t.Fatal(err)
	}
	held, err := atomicfile.OpenReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	defer held.Close()
	if err := atomicfile.Write(path, newBytes, 0o600); err != nil {
		t.Fatalf("held agreement reader blocked atomic publication: %v", err)
	}
	old, err := io.ReadAll(held)
	if err != nil || !bytes.Equal(old, oldBytes) {
		t.Fatalf("reader did not retain exact old snapshot: %q, %v", old, err)
	}
	next, err := readDependencyDocument(path, maxDependencyLockBytes)
	if err != nil || !bytes.Equal(next, newBytes) {
		t.Fatalf("bounded agreement loader did not observe exact new snapshot: %q, %v", next, err)
	}
}
