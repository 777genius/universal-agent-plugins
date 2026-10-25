package domain

import (
	"fmt"
	"path/filepath"
	"strings"
)

// TargetContext separates the selected project from the original user profile.
// These operational locators never contribute to portable source identity.
type TargetContext struct {
	Scope          InstallScope `json:"-"`
	ProjectRoot    string       `json:"-"`
	UserConfigRoot string       `json:"-"`
}

// NewTargetContext validates explicit locators without observing or creating
// directories. The composition boundary canonicalizes the selected directory.
func NewTargetContext(scope InstallScope, projectRoot, userConfigRoot string) (TargetContext, error) {
	if scope == "" {
		scope = ScopeUser
	}
	target := TargetContext{Scope: scope, ProjectRoot: projectRoot, UserConfigRoot: userConfigRoot}
	if scope != ScopeUser && scope != ScopeProject && scope != ScopeLocal {
		return TargetContext{}, fmt.Errorf("unsupported installation scope %q", scope)
	}
	if userConfigRoot != "" && !cleanTargetRoot(userConfigRoot) {
		return TargetContext{}, fmt.Errorf("user configuration root must be absolute and clean")
	}
	if scope != ScopeProject {
		if projectRoot != "" {
			return TargetContext{}, fmt.Errorf("project root requires project scope")
		}
		return target, nil
	}
	if !cleanTargetRoot(projectRoot) || filepath.Dir(projectRoot) == projectRoot {
		return TargetContext{}, fmt.Errorf("project scope requires an explicit absolute clean non-root directory")
	}
	return target, nil
}

func cleanTargetRoot(root string) bool {
	return root != "" && strings.TrimSpace(root) == root && filepath.IsAbs(root) &&
		filepath.Clean(root) == root && !strings.ContainsAny(root, "\x00\r\n")
}
