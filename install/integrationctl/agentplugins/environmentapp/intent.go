// Package environmentapp applies reviewed dependency environment changes without
// depending on a command, UI, concrete filesystem or client implementation.
package environmentapp

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/environment"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/installer"
)

// IntentFiles accesses only the manifest/lock pair. Write must preserve the
// host's existing atomic-file and permission policy.
type IntentFiles interface {
	Read(string) ([]byte, error)
	Write(string, []byte) error
}

// Intent retains the reviewed preimages and optional replacement bytes. A nil
// preimage means absent; a nil candidate means leave that file unchanged.
// Successful publication advances the preimages used by subsequent binding
// guards. Callers must retain this same object throughout application.
type Intent struct {
	ManifestPath, LockPath           string
	ManifestBytes, LockBytes         []byte
	CandidateManifest, CandidateLock []byte
}

func (intent Intent) Check(files IntentFiles) error {
	if !sameFile(files, intent.ManifestPath, intent.ManifestBytes) || !sameFile(files, intent.LockPath, intent.LockBytes) {
		return installer.ErrPlanChanged
	}
	return nil
}

// Pending reports whether publication would change either reviewed file.
func (intent Intent) Pending() bool {
	return intent.CandidateManifest != nil && !bytes.Equal(intent.CandidateManifest, intent.ManifestBytes) ||
		intent.CandidateLock != nil && !bytes.Equal(intent.CandidateLock, intent.LockBytes)
}

// Publish preserves manifest-first publication and its existing recovery
// guidance. The caller holds the environment lock and has rechecked the review.
// A failed write can still have changed disk; Service observes that separately.
func (intent *Intent) Publish(files IntentFiles) error {
	if intent.CandidateManifest != nil && !bytes.Equal(intent.CandidateManifest, intent.ManifestBytes) {
		if err := files.Write(intent.ManifestPath, intent.CandidateManifest); err != nil {
			return err
		}
		intent.ManifestBytes = intent.CandidateManifest
	}
	if intent.CandidateLock != nil && !bytes.Equal(intent.CandidateLock, intent.LockBytes) {
		if err := files.Write(intent.LockPath, intent.CandidateLock); err != nil {
			return fmt.Errorf("uap.toml saved, uap.lock not written: %w; run agentplugins deps lock, then retry, or agentplugins deps sync", err)
		}
		intent.LockBytes = intent.CandidateLock
	}
	return nil
}

func (intent Intent) snapshot() Intent {
	intent.ManifestBytes = append([]byte(nil), intent.ManifestBytes...)
	intent.LockBytes = append([]byte(nil), intent.LockBytes...)
	return intent
}

func sameFile(files IntentFiles, path string, expected []byte) bool {
	body, err := files.Read(path)
	if errors.Is(err, fs.ErrNotExist) {
		return expected == nil
	}
	return err == nil && bytes.Equal(body, expected)
}

// SelectedLock limits materialization to the already-selected closure. It does
// not change the prepared graph, lock metadata or binding authority.
func SelectedLock(lock environment.Lock, aliases []string) environment.Lock {
	if len(aliases) == 0 {
		return lock
	}
	selected := make(map[string]bool, len(aliases))
	for _, alias := range aliases {
		selected[alias] = true
	}
	packages := lock.Packages
	lock.Packages = nil
	for _, pkg := range packages {
		if selected[pkg.Alias] {
			lock.Packages = append(lock.Packages, pkg)
		}
	}
	return lock
}
