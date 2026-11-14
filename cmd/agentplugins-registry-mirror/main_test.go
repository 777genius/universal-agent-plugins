package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEnforceMonotonicAllowsAdvanceAndRejectsRollback(t *testing.T) {
	previous := mirrorMetadata{
		SchemaVersion:      1,
		RegistryRepository: defaultRegistry,
		DirectorySequence:  10,
		DirectoryDigest:    "sha256:" + strings.Repeat("a", 64),
		DiscoverySequence:  8,
		DiscoveryDigest:    "sha256:" + strings.Repeat("b", 64),
	}
	path := filepath.Join(t.TempDir(), "MIRROR_METADATA.json")
	body := []byte(`{
  "schema_version": 1,
  "registry_repository": "777genius/universal-agent-plugins-registry",
  "directory_sequence": 10,
  "directory_digest": "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
  "directory_source_commit": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
  "discovery_sequence": 8,
  "discovery_digest": "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
  "discovery_source_commit": "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
  "generated_files": 10
}
`)
	if err := os.WriteFile(path, body, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := enforceMonotonic(path, mirrorMetadata{
		SchemaVersion:      1,
		RegistryRepository: defaultRegistry,
		DirectorySequence:  11,
		DirectoryDigest:    previous.DirectoryDigest,
		DiscoverySequence:  9,
		DiscoveryDigest:    previous.DiscoveryDigest,
	}); err != nil {
		t.Fatalf("advance rejected: %v", err)
	}
	if err := enforceMonotonic(path, mirrorMetadata{
		SchemaVersion:      1,
		RegistryRepository: defaultRegistry,
		DirectorySequence:  9,
		DirectoryDigest:    previous.DirectoryDigest,
		DiscoverySequence:  9,
		DiscoveryDigest:    previous.DiscoveryDigest,
	}); err == nil || !strings.Contains(err.Error(), "regresses") {
		t.Fatalf("rollback was not rejected: %v", err)
	}
}

func TestEnforceMonotonicRejectsSameSequenceConflict(t *testing.T) {
	path := filepath.Join(t.TempDir(), "MIRROR_METADATA.json")
	if err := os.WriteFile(path, []byte(`{
  "schema_version": 1,
  "registry_repository": "777genius/universal-agent-plugins-registry",
  "directory_sequence": 10,
  "directory_digest": "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
  "directory_source_commit": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
  "discovery_sequence": 8,
  "discovery_digest": "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
  "discovery_source_commit": "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
  "generated_files": 10
}
`), 0o600); err != nil {
		t.Fatal(err)
	}
	err := enforceMonotonic(path, mirrorMetadata{
		SchemaVersion:      1,
		RegistryRepository: defaultRegistry,
		DirectorySequence:  10,
		DirectoryDigest:    "sha256:" + strings.Repeat("c", 64),
		DiscoverySequence:  8,
		DiscoveryDigest:    "sha256:" + strings.Repeat("b", 64),
	})
	if err == nil || !strings.Contains(err.Error(), "conflicting") {
		t.Fatalf("same-sequence conflict was not rejected: %v", err)
	}
}

func TestEnforceMonotonicProtectsSecurityFeed(t *testing.T) {
	path := filepath.Join(t.TempDir(), "MIRROR_METADATA.json")
	if err := os.WriteFile(path, []byte(`{
  "schema_version": 1,
  "registry_repository": "777genius/universal-agent-plugins-registry",
  "directory_sequence": 10,
  "directory_digest": "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
  "directory_source_commit": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
  "discovery_sequence": 8,
  "discovery_digest": "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
  "discovery_source_commit": "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
  "security_sequence": 3,
  "security_digest": "sha256:cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc",
  "security_source_commit": "cccccccccccccccccccccccccccccccccccccccc",
  "generated_files": 13
}
`), 0o600); err != nil {
		t.Fatal(err)
	}
	base := mirrorMetadata{
		SchemaVersion:      1,
		RegistryRepository: defaultRegistry,
		DirectorySequence:  10,
		DirectoryDigest:    "sha256:" + strings.Repeat("a", 64),
		DiscoverySequence:  8,
		DiscoveryDigest:    "sha256:" + strings.Repeat("b", 64),
		SecuritySequence:   3,
		SecurityDigest:     "sha256:" + strings.Repeat("c", 64),
	}
	rollback := base
	rollback.SecuritySequence = 2
	if err := enforceMonotonic(path, rollback); err == nil || !strings.Contains(err.Error(), "regresses") {
		t.Fatalf("Security rollback was not rejected: %v", err)
	}
	conflict := base
	conflict.SecurityDigest = "sha256:" + strings.Repeat("d", 64)
	if err := enforceMonotonic(path, conflict); err == nil || !strings.Contains(err.Error(), "Security sequence") {
		t.Fatalf("Security same-sequence conflict was not rejected: %v", err)
	}
}

func TestWriteStagedRejectsTraversal(t *testing.T) {
	for _, relative := range []string{"../escape", "/absolute", "a\\b", "a/../../escape"} {
		if err := writeStaged(t.TempDir(), relative, []byte("x")); err == nil {
			t.Fatalf("unsafe path %q accepted", relative)
		}
	}
}

func TestStagedFilesFollowSignedPointerPaths(t *testing.T) {
	const stem = "00000000000000000027"
	files := stagedFiles(
		directoryResult{stem: stem},
		discoveryResult{stem: stem},
		securityResult{stem: stem},
	)
	paths := make(map[string]bool, len(files))
	for _, file := range files {
		paths[file.rel] = true
	}
	for _, expected := range []string{
		"registry/schemas/1/latest.json",
		"registry/schemas/1/snapshots/" + stem + ".json",
		"registry/schemas/1/snapshots/" + stem + ".envelope.json",
		"discovery/latest.json",
		"discovery/snapshots/" + stem + ".json",
		"discovery/snapshots/" + stem + ".envelope.json",
		"discovery/search/" + stem + ".json",
		"security/latest.json",
		"security/snapshots/" + stem + ".json",
		"security/snapshots/" + stem + ".envelope.json",
	} {
		if !paths[expected] {
			t.Fatalf("signed pointer artifact path is missing: %s", expected)
		}
	}
	for _, unexpected := range []string{
		"registry/schemas/1/" + stem + ".json",
		"registry/schemas/1/" + stem + ".envelope.json",
		"discovery/" + stem + ".json",
		"discovery/" + stem + ".envelope.json",
		"security/" + stem + ".json",
		"security/" + stem + ".envelope.json",
	} {
		if paths[unexpected] {
			t.Fatalf("legacy root artifact path must not be emitted: %s", unexpected)
		}
	}
}
