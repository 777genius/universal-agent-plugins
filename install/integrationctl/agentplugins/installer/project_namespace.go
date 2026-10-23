package installer

import (
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
	if e.cfg.ProjectRoot == "" {
		return nil
	}
	e.projectGuard.mu.Lock()
	defer e.projectGuard.mu.Unlock()
	root := e.cfg.ProjectRoot
	if e.cfg.StateRoot != filepath.Join(root, ".uap") {
		return fmt.Errorf("project mutable namespace must be the selected project's .uap directory")
	}
	project, err := os.Lstat(root)
	if err != nil || !project.IsDir() || project.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("project root must remain an existing real directory")
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
	namespace, err := os.Lstat(e.cfg.StateRoot)
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
