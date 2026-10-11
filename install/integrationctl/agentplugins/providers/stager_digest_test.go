package providers

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/ports"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/packagesnapshot"
)

// Verification must read fresh bytes without writable snapshot storage.
func TestStagerVerifyDigestWithoutSnapshotOutput(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	path := filepath.Join(root, "file")
	writeTestFile(t, path, "one")
	snapshot, err := (packagesnapshot.Builder{}).Build(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	defer snapshot.Close()
	output := t.TempDir()
	stager := Stager{SnapshotBuilder: packagesnapshot.Builder{TempRoot: output}}
	for range 2 {
		if err := stager.Verify(context.Background(), root, snapshot.Digest); err != nil {
			t.Fatal(err)
		}
	}
	entries, err := os.ReadDir(output)
	if err != nil || len(entries) != 0 {
		t.Fatalf("verification output = %v, %v", entries, err)
	}
	stager.SnapshotBuilder.TempRoot = path
	if err := stager.Verify(context.Background(), root, snapshot.Digest); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, path, "two")
	want, err := (packagesnapshot.Builder{}).Digest(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	verifyErr := requireVerificationKind(t, stager.Verify(context.Background(), root, snapshot.Digest), ports.VerificationDigestMismatch)
	if verifyErr.ActualDigest != want || want == snapshot.Digest {
		t.Fatalf("mismatch digest = %q, want %q", verifyErr.ActualDigest, want)
	}
}

func TestStagerVerifyDigestErrorClasses(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "file"), "data")
	stager := Stager{SnapshotBuilder: packagesnapshot.Builder{Limits: packagesnapshot.Limits{MaxFileBytes: 1}}}
	requireVerificationKind(t, stager.Verify(context.Background(), root, "sha256:expected"), ports.VerificationIndeterminate)
	requireVerificationKind(t, stager.Verify(context.Background(), filepath.Join(root, "absent"), "sha256:expected"), ports.VerificationAbsent)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	verifyErr := requireVerificationKind(t, stager.Verify(ctx, root, "sha256:expected"), ports.VerificationIndeterminate)
	if !errors.Is(verifyErr, context.Canceled) {
		t.Fatalf("canceled error = %v", verifyErr)
	}
	for _, marker := range []string{".git", ".plugin-kit-ai.lock"} {
		marked := t.TempDir()
		writeTestFile(t, filepath.Join(marked, "nested", marker), "marker")
		requireVerificationKind(t, stager.Verify(context.Background(), marked, "sha256:expected"), ports.VerificationExcludedMarker)
	}
}

func requireVerificationKind(t *testing.T, err error, kind ports.VerificationKind) *ports.VerificationError {
	t.Helper()
	var verifyErr *ports.VerificationError
	if !errors.As(err, &verifyErr) || verifyErr.Kind != kind {
		t.Fatalf("Verify error = %v, want %s", err, kind)
	}
	return verifyErr
}
