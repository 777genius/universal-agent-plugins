package safemutate

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

func TestMutateFileRollsBackOnPostValidationFailure(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	path := filepath.Join(root, "config.json")
	if err := os.WriteFile(path, []byte("{\"before\":true}\n"), 0o644); err != nil {
		t.Fatalf("seed file: %v", err)
	}

	_, err := OS{}.MutateFile(context.Background(), ports.SafeFileMutationInput{
		Path: path,
		Mode: 0o644,
		Build: func(original []byte, exists bool) ([]byte, error) {
			if !exists {
				t.Fatal("expected original file to exist")
			}
			return []byte("{\"after\":true}\n"), nil
		},
		ValidateAfter: func(context.Context, string, []byte) error {
			return errors.New("forced verify failure")
		},
	})
	if err == nil {
		t.Fatal("expected mutation to fail")
	}

	body, readErr := os.ReadFile(path)
	if readErr != nil {
		t.Fatalf("read rolled back file: %v", readErr)
	}
	if got, want := string(body), "{\"before\":true}\n"; got != want {
		t.Fatalf("body = %q, want %q", got, want)
	}
}

func TestMutateFileCreatesFileWhenMissing(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	path := filepath.Join(root, "config.json")

	result, err := OS{}.MutateFile(context.Background(), ports.SafeFileMutationInput{
		Path: path,
		Mode: 0o644,
		Build: func(original []byte, exists bool) ([]byte, error) {
			if exists {
				t.Fatal("expected file to be absent")
			}
			if len(original) != 0 {
				t.Fatalf("original = %q, want empty", string(original))
			}
			return []byte("{\"created\":true}\n"), nil
		},
	})
	if err != nil {
		t.Fatalf("mutate file: %v", err)
	}
	if result.HadOriginal {
		t.Fatal("HadOriginal = true, want false")
	}
	body, readErr := os.ReadFile(path)
	if readErr != nil {
		t.Fatalf("read created file: %v", readErr)
	}
	if got, want := string(body), "{\"created\":true}\n"; got != want {
		t.Fatalf("body = %q, want %q", got, want)
	}
}

func TestMutateFileRemovesBackupAfterSuccessfulWrite(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	path := filepath.Join(root, "config.json")
	if err := os.WriteFile(path, []byte("{\"before\":true}\n"), 0o644); err != nil {
		t.Fatalf("seed file: %v", err)
	}

	result, err := OS{}.MutateFile(context.Background(), ports.SafeFileMutationInput{
		Path: path,
		Mode: 0o644,
		Build: func(original []byte, exists bool) ([]byte, error) {
			if !exists {
				t.Fatal("expected original file to exist")
			}
			return []byte("{\"after\":true}\n"), nil
		},
	})
	if err != nil {
		t.Fatalf("mutate file: %v", err)
	}
	if result.BackupPath != "" {
		t.Fatalf("backup path = %q, want empty after successful cleanup", result.BackupPath)
	}
	matches, err := filepath.Glob(filepath.Join(root, "config.json.bak-*"))
	if err != nil {
		t.Fatalf("glob backups: %v", err)
	}
	if len(matches) != 0 {
		t.Fatalf("backup files left behind: %#v", matches)
	}
}
