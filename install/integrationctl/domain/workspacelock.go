package domain

// WorkspaceLockFileName is the workspace desired-integrations lock written at
// the workspace root.
const WorkspaceLockFileName = ".uap-integrations.lock"

// LegacyWorkspaceLockFileName is the retired plugin-kit-ai-era name of the
// workspace lock. It stays excluded from package trees and ownership checks so
// workspaces that still carry it do not get digest drift, but it is never
// written and never read silently: see adapters/workspacelock.
const LegacyWorkspaceLockFileName = ".plugin-kit-ai.lock"

// IsWorkspaceLockFile reports whether base names the workspace lock under its
// current or legacy name. Callers keep their own directory checks.
func IsWorkspaceLockFile(base string) bool {
	return base == WorkspaceLockFileName || base == LegacyWorkspaceLockFileName
}
