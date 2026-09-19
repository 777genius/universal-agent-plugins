// Package installer is the public UAP installer API for standard local Agent
// Plugins packages. Callers must not import raw Store or Kernel types through
// this package; composition stays inside New.
// Inspect reports pending journals and unfinished state receipts without
// recovering them. Recover takes that observation and refuses a changed scope.
// Request.Targets selects Claude and Codex in one operation; Switch remains unpublished.
package installer
