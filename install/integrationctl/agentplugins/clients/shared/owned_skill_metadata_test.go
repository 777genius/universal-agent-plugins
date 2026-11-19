package shared

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/packagesnapshot"
)

type ownedSkillMetadataIO struct {
	nativeconfig.FileIO
	read func(string, int64) ([]byte, os.FileMode, bool, error)
}

func (files ownedSkillMetadataIO) ReadNoFollowBounded(path string, limit int64) ([]byte, os.FileMode, bool, error) {
	return files.read(path, limit)
}

func ownedSkillMetadataFixture(t *testing.T) (string, []byte, string) {
	t.Helper()
	root := t.TempDir()
	body := []byte("---\nname: owned\ndescription: Original\n---\nOriginal instructions.\n")
	for name, value := range map[string][]byte{"SKILL.md": body, "resource.txt": []byte("original resource\n")} {
		if err := os.WriteFile(filepath.Join(root, name), value, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	digest, err := DigestSkillDirectory(root)
	if err != nil {
		t.Fatal(err)
	}
	return root, body, digest
}

func ownedSkillMetadataDiskRead(path string, limit int64) ([]byte, os.FileMode, bool, error) {
	file, err := nativeconfig.New().ReadExactFileBounded(path, limit)
	return file.Body, file.Mode, file.Exists, err
}

func TestOwnedSkillMetadataUsesSealedBytesDuringLiveMetadataABA(t *testing.T) {
	root, original, digest := ownedSkillMetadataFixture(t)
	path := filepath.Join(root, "SKILL.md")
	var sealed string
	files := ownedSkillMetadataIO{read: func(selected string, limit int64) ([]byte, os.FileMode, bool, error) {
		sealed = selected
		if selected == path || limit != 1<<20 {
			t.Fatal("metadata did not use the bounded sealed Skill path")
		}
		// The live file can be B only while metadata is being read, then A
		// again at the final digest. Its temporary metadata must never qualify.
		if err := os.WriteFile(path, []byte("substituted instructions"), 0o600); err != nil {
			t.Fatal(err)
		}
		body, mode, exists, err := ownedSkillMetadataDiskRead(selected, limit)
		if restore := os.WriteFile(path, original, 0o600); restore != nil {
			t.Fatal(restore)
		}
		return body, mode, exists, err
	}}
	body, err := ReadOwnedSkillMetadata(t.Context(), nativeconfig.NewWithFileIO(files), root, digest)
	if err != nil || !bytes.Equal(body, original) {
		t.Fatalf("live ABA substituted unowned metadata: %q %v", body, err)
	}
	if _, err := os.Lstat(filepath.Dir(sealed)); !os.IsNotExist(err) {
		t.Fatalf("sealed Skill was not cleaned: %v", err)
	}
}

func TestOwnedSkillMetadataRefusesDigestOrLateResourceChange(t *testing.T) {
	for _, mode := range []string{"expected-digest", "resource"} {
		t.Run(mode, func(t *testing.T) {
			root, _, digest := ownedSkillMetadataFixture(t)
			reads := 0
			files := ownedSkillMetadataIO{read: func(path string, limit int64) ([]byte, os.FileMode, bool, error) {
				reads++
				if err := os.WriteFile(filepath.Join(root, "resource.txt"), []byte("changed resource"), 0o600); err != nil {
					t.Fatal(err)
				}
				return ownedSkillMetadataDiskRead(path, limit)
			}}
			if mode == "expected-digest" {
				digest = "sha256:not-the-owned-digest"
			}
			body, err := ReadOwnedSkillMetadata(t.Context(), nativeconfig.NewWithFileIO(files), root, digest)
			if err == nil || body != nil || (mode == "expected-digest" && reads != 0) || (mode == "resource" && reads != 1) {
				t.Fatalf("unowned or changed Skill qualified: body=%q reads=%d err=%v", body, reads, err)
			}
		})
	}
}

func TestOwnedSkillMetadataRequiresBoundedReaderBeforeSnapshot(t *testing.T) {
	// A nonexistent source would make Build fail differently. The explicit
	// injected capability refusal must win without opening that source.
	kernel := nativeconfig.NewWithFileIO(struct{ nativeconfig.FileIO }{})
	body, err := ReadOwnedSkillMetadata(t.Context(), kernel, filepath.Join(t.TempDir(), "absent"), "unowned")
	if !errors.Is(err, nativeconfig.ErrBoundedReadUnavailable) || body != nil {
		t.Fatalf("unsupported backend acquired snapshot authority: %q %v", body, err)
	}
}

func TestOwnedSkillMetadataCloseFailureCannotReturnMetadata(t *testing.T) {
	root, _, digest := ownedSkillMetadataFixture(t)
	snapshot, err := (packagesnapshot.Builder{}).Build(t.Context(), root)
	if err != nil {
		t.Fatal(err)
	}
	cause := errors.New("controlled snapshot cleanup refusal")
	closed := 0
	closeSnapshot := func() error {
		closed++
		return errors.Join(snapshot.Close(), cause)
	}
	body, err := readOwnedSkillSnapshot(t.Context(), nativeconfig.New(), root, digest, snapshot.Root, closeSnapshot)
	if !errors.Is(err, cause) || body != nil || closed != 1 {
		t.Fatalf("cleanup failure qualified metadata: %q closes=%d err=%v", body, closed, err)
	}
}

func TestOwnedSkillMetadataCancellationAndReadRefusalStayClosed(t *testing.T) {
	for _, mode := range []string{"cancel", "read-error", "missing"} {
		t.Run(mode, func(t *testing.T) {
			root, _, digest := ownedSkillMetadataFixture(t)
			cause := errors.New("injected bounded reader refused")
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			files := ownedSkillMetadataIO{read: func(string, int64) ([]byte, os.FileMode, bool, error) {
				if mode == "cancel" {
					cancel()
					return []byte("ignored"), 0o400, true, nil
				}
				if mode == "missing" {
					return nil, 0o400, false, nil
				}
				return nil, 0, false, cause
			}}
			body, err := ReadOwnedSkillMetadata(ctx, nativeconfig.NewWithFileIO(files), root, digest)
			if err == nil || body != nil || (mode == "cancel" && !errors.Is(err, context.Canceled)) || (mode == "read-error" && !errors.Is(err, cause)) {
				t.Fatalf("bounded refusal/cancellation qualified: %q %v", body, err)
			}
		})
	}
}
