package agentpluginscli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/environment"
)

// Both JSON decoders and TOML entry points must reject oversized input before
// parsing or interpreting any source identity, including caller-owned bytes.
func TestDependencyAgreementOversizedBytesFailClosed(t *testing.T) {
	requirements := environment.Requirements{SchemaVersion: environment.ManifestSchemaVersion}
	for _, compatible := range []bool{false, true} {
		hostile := bytes.Repeat([]byte(" "), int(maxDependencyLockBytes)+1)
		if _, err := decodeDependencyLockBytes(hostile, requirements, compatible); err == nil || !strings.Contains(err.Error(), "exceeds") {
			t.Fatalf("compatible=%t oversized lock accepted: %v", compatible, err)
		}
	}
	if _, err := readDependencyManifestBytes("uap.toml", bytes.Repeat([]byte(" "), int(maxDependencyManifestBytes)+1)); err == nil || !strings.Contains(err.Error(), "exceeds") {
		t.Fatalf("oversized TOML admitted: %v", err)
	}
}

func TestDependencyAgreementDiskBoundaryRejectsNonRegularAndOversized(t *testing.T) {
	root := t.TempDir()
	requirements := environment.Requirements{SchemaVersion: environment.ManifestSchemaVersion}
	for _, tc := range []struct {
		name  string
		limit int64
	}{
		{"uap.toml", maxDependencyManifestBytes},
		{"uap.lock", maxDependencyLockBytes},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(root, tc.name)
			file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_RDWR, 0o600)
			if err != nil {
				t.Fatal(err)
			}
			if err := file.Truncate(tc.limit + 1); err != nil {
				t.Fatal(err)
			}
			if err := file.Close(); err != nil {
				t.Fatal(err)
			}
			before, err := os.Stat(path)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := readDependencyDocument(path, tc.limit); err == nil || !strings.Contains(err.Error(), "exceeds") {
				t.Fatalf("oversized agreement accepted: %v", err)
			}
			if tc.name == "uap.lock" {
				for _, reader := range []func(string, environment.Requirements) (environment.Lock, error){readDependencyLock, readCompatibleDependencyLock} {
					if _, err := reader(path, requirements); err == nil || !strings.Contains(err.Error(), "exceeds") {
						t.Fatalf("oversized reader accepted: %v", err)
					}
				}
			} else {
				if _, err := readDependencyManifest(path); err == nil || !strings.Contains(err.Error(), "exceeds") {
					t.Fatalf("oversized manifest accepted: %v", err)
				}
			}
			after, err := os.Stat(path)
			if err != nil || !os.SameFile(before, after) || before.Size() != after.Size() || !before.ModTime().Equal(after.ModTime()) {
				t.Fatalf("refusal mutated source agreement: %v", err)
			}
		})
	}
	if _, err := readDependencyDocument(root, maxDependencyManifestBytes); err == nil || !strings.Contains(err.Error(), "regular") {
		t.Fatalf("directory agreement accepted: %v", err)
	}
}

// Reviewed intent reads and writes must honor the same admission policy as
// initial CLI loading, even when a file is replaced during an operation.
func TestDependencyIntentFilesRejectOversizedWriteWithoutMutation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "uap.lock")
	original := []byte("pinned authority; do not replace")
	if err := os.WriteFile(path, original, 0o600); err != nil {
		t.Fatal(err)
	}
	before, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	files := dependencyIntentFiles{}
	if err := files.Write(path, bytes.Repeat([]byte("a"), int(maxDependencyLockBytes)+1)); err == nil || !strings.Contains(err.Error(), "exceeds") {
		t.Fatalf("oversized new lock was published: %v", err)
	}
	after, err := os.Stat(path)
	if err != nil || !os.SameFile(before, after) || !before.ModTime().Equal(after.ModTime()) {
		t.Fatalf("failed write changed existing lock metadata: %v", err)
	}
	got, err := files.Read(path)
	if err != nil || !bytes.Equal(got, original) {
		t.Fatalf("failed write altered lock bytes: %v", err)
	}
	if _, err := files.Read(filepath.Join(t.TempDir(), "unexpected.json")); err == nil || !strings.Contains(err.Error(), "unsupported") {
		t.Fatalf("unknown authority filename accepted: %v", err)
	}
}

func TestDependencyCandidateSizePreflightProtectsManifestAndLock(t *testing.T) {
	root := t.TempDir()
	manifest := filepath.Join(root, "uap.toml")
	lock := filepath.Join(root, "uap.lock")
	const sentinel = "existing agreement; preserve bytes"
	for _, path := range []string{manifest, lock} {
		if err := os.WriteFile(path, []byte(sentinel), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	for _, tc := range []struct {
		name string
		manifest, lock []byte
		want string
	}{
		{"normal", []byte("schema_version = 1"), []byte("{}"), ""},
		{"manifest-too-large", bytes.Repeat([]byte("a"), int(maxDependencyManifestBytes)+1), []byte("{}"), "uap.toml"},
		{"lock-too-large", []byte("schema_version = 1"), bytes.Repeat([]byte("a"), int(maxDependencyLockBytes)+1), "uap.lock"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := validateDependencyCandidateFiles(tc.manifest, tc.lock)
			if tc.want == "" && err != nil || tc.want != "" && (err == nil || !strings.Contains(err.Error(), tc.want)) {
				t.Fatalf("candidate admission wrong: %v", err)
			}
			for _, path := range []string{manifest, lock} {
				body, readErr := os.ReadFile(path)
				if readErr != nil || string(body) != sentinel {
					t.Fatalf("candidate admission mutated %s: %v", path, readErr)
				}
			}
		})
	}
}

func TestDependencyManagementRecoveryReaderIsBounded(t *testing.T) {
	for _, name := range []string{"uap.toml", "uap.lock"} {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), name)
			limit, err := dependencyDocumentLimit(path)
			if err != nil {
				t.Fatal(err)
			}
			file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_RDWR, 0o600)
			if err != nil {
				t.Fatal(err)
			}
			if err := file.Truncate(limit + 1); err != nil {
				t.Fatal(err)
			}
			if err := file.Close(); err != nil {
				t.Fatal(err)
			}
			if _, err := managementReadOptional(path); err == nil || !strings.Contains(err.Error(), "exceeds") {
				t.Fatalf("management recovery admitted oversized agreement: %v", err)
			}
		})
	}
	missing := filepath.Join(t.TempDir(), "uap.lock")
	if body, err := managementReadOptional(missing); err != nil || body != nil {
		t.Fatalf("missing optional recovery agreement changed: %v, %q", err, body)
	}
}

func TestDependencyAgreementReaderRejectsSymlinkAndKeepsTarget(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "foreign.toml")
	const data = "foreign secret; never change or expose"
	if err := os.WriteFile(target, []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "uap.toml")
	if err := os.Symlink(target, link); err != nil {
		// Some Windows runners prohibit creating symlinks. The nonregular
		// directory refusal is still exercised on all four E2E platforms.
		t.Logf("symlink creation unavailable: %v", err)
		return
	}
	if _, err := readDependencyManifest(link); err == nil || !strings.Contains(err.Error(), "non-symlink") || strings.Contains(err.Error(), data) {
		t.Fatalf("symlink authority was followed or leaked: %v", err)
	}
	body, err := os.ReadFile(target)
	if err != nil || string(body) != data {
		t.Fatalf("foreign symlink target was modified: %q, %v", body, err)
	}
}
