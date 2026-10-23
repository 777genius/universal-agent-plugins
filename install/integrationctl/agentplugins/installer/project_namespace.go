package installer

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/pathpolicy"
)

type projectNamespaceGuard struct {
	mu                 sync.Mutex
	project, namespace os.FileInfo
}

// CheckProjectNamespace validates the selected physical namespace without
// creating it. The CLI reuses this guard before environment locks and writes.
// Existing roots are pinned in memory and revalidated before apply/recovery.
func (e *Engine) CheckProjectNamespace() error {
	if e == nil {
		return fmt.Errorf("%w: installer engine is required", ErrInvalidConfig)
	}
	if e.cfg.ProjectRoot == "" {
		return nil
	}
	if e.projectGuard == nil {
		return fmt.Errorf("%w: project namespace guard is not configured", ErrInvalidConfig)
	}
	e.projectGuard.mu.Lock()
	defer e.projectGuard.mu.Unlock()
	return e.checkProjectNamespaceLocked()
}

func (e *Engine) checkProjectNamespaceLocked() error {
	root := e.cfg.ProjectRoot
	if e.cfg.StateRoot != filepath.Join(root, ".uap") {
		return fmt.Errorf("project mutable namespace must be the selected project's .uap directory")
	}
	project, err := projectDirectoryIdentity(root)
	if err != nil {
		return fmt.Errorf("project root must remain an existing real directory: %w", err)
	}
	if previous := e.projectGuard.project; previous != nil && !os.SameFile(previous, project) {
		return fmt.Errorf("%w: selected project directory identity changed", ErrPlanChanged)
	}
	if err := pathpolicy.RequireContainedChild(root, e.cfg.StateRoot); err != nil {
		return fmt.Errorf("unsafe project namespace: %w", err)
	}
	for _, child := range []string{e.cfg.StateFile, e.cfg.LockFile, e.cfg.OperationsDir, e.cfg.PluginDataBase, e.cfg.ManagedRoot} {
		if err := pathpolicy.RequireContainedChild(e.cfg.StateRoot, child); err != nil {
			return fmt.Errorf("unsafe project mutable path: %w", err)
		}
	}
	if !e.cfg.temporaryRoot {
		if err := pathpolicy.RequireContainedChild(e.cfg.StateRoot, e.cfg.TempRoot); err != nil {
			return fmt.Errorf("unsafe project temporary path: %w", err)
		}
	}
	namespace, err := projectDirectoryIdentity(e.cfg.StateRoot)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if namespace != nil && !namespace.IsDir() {
		return fmt.Errorf("project mutable namespace is not a directory")
	}
	if previous := e.projectGuard.namespace; previous != nil &&
		(namespace == nil || !os.SameFile(previous, namespace)) {
		return fmt.Errorf("%w: project mutable namespace identity changed", ErrPlanChanged)
	}
	e.projectGuard.project, e.projectGuard.namespace = project, namespace
	return nil
}

// Windows Lstat defers file-ID lookup until SameFile and reopens its old path.
// Capture the identity from an open handle before retaining it across renames.
func projectDirectoryIdentity(path string) (os.FileInfo, error) {
	before, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !before.IsDir() || before.Mode()&os.ModeSymlink != 0 {
		return nil, fmt.Errorf("project directory is not an exact real directory")
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	identity, statErr := file.Stat()
	closeErr := file.Close()
	if err := errors.Join(statErr, closeErr); err != nil {
		return nil, err
	}
	if !identity.IsDir() || !os.SameFile(before, identity) {
		return nil, fmt.Errorf("%w: project directory changed while capturing identity", ErrPlanChanged)
	}
	return identity, nil
}
