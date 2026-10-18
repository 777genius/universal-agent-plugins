// Package qoder implements the native Qoder hook runtime codec for the proven
// four-event subset (SessionStart, UserPromptSubmit, PostToolUse, Stop) of the
// Claude-derived hook protocol used by Qoder plugin bundles: the hook binary is
// invoked as <command> <kebab-subcommand> with the event JSON on stdin and
// returns a Claude-style response JSON on stdout.
package qoder
