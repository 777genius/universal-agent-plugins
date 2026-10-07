// Package installer is the public UAP installer API for standard local Agent
// Plugins packages. Callers must not import raw Store or Kernel types through
// this package; composition stays inside New.
// Inspect reports pending native intents, journals and unfinished receipts without
// recovering them. Recover takes that observation and refuses a changed scope.
// Request.Targets selects two distinct registered clients. Historical group removal
// retains Claude/Codex limits; selected native removal needs its typed capability.
// Switch remains unpublished.
package installer
