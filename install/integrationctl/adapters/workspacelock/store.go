package workspacelock

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
	"gopkg.in/yaml.v3"
)

type Store struct {
	FS   ports.FileSystem
	File string
}

func (s Store) Path() string {
	return s.File
}

// legacyNamePresent refuses a workspace lock under its retired name instead of
// silently adopting or ignoring it: the file is workspace state, and renaming
// is the user's explicit action.
func (s Store) legacyNamePresent(ctx context.Context) error {
	legacy := filepath.Join(filepath.Dir(s.File), domain.LegacyWorkspaceLockFileName)
	if _, err := s.FS.ReadFile(ctx, legacy); err != nil {
		return nil
	}
	return fmt.Errorf("workspace lock uses the legacy name %q; rename it to %q (for example: mv %s %s)",
		domain.LegacyWorkspaceLockFileName, domain.WorkspaceLockFileName, legacy, s.File)
}

func (s Store) Load(ctx context.Context) (domain.WorkspaceLock, error) {
	if s.File == "" {
		return domain.WorkspaceLock{}, errors.New("workspace lock path required")
	}
	body, err := s.FS.ReadFile(ctx, s.File)
	if err != nil {
		if legacyErr := s.legacyNamePresent(ctx); legacyErr != nil {
			return domain.WorkspaceLock{}, legacyErr
		}
		return domain.WorkspaceLock{}, err
	}
	var out domain.WorkspaceLock
	if err := yaml.Unmarshal(body, &out); err != nil {
		return domain.WorkspaceLock{}, err
	}
	if out.APIVersion == "" {
		out.APIVersion = "v1"
	}
	return out, nil
}

func (s Store) Save(ctx context.Context, lock domain.WorkspaceLock) error {
	if s.File == "" {
		return errors.New("workspace lock path required")
	}
	if lock.APIVersion == "" {
		lock.APIVersion = "v1"
	}
	body, err := yaml.Marshal(lock)
	if err != nil {
		return err
	}
	if err := s.FS.MkdirAll(ctx, filepath.Dir(s.File), 0o755); err != nil {
		return err
	}
	return s.FS.WriteFileAtomic(ctx, s.File, body, 0o644)
}

func IsNotExist(err error) bool {
	return os.IsNotExist(err)
}
