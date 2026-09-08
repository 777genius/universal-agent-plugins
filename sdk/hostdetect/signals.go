package hostdetect

// DefaultRegistry returns a fresh copy of the built-in signal order:
// Codex before Claude. Callers may mutate the returned slice freely;
// package state is never shared.
//
// No env marker is registered for either platform: Codex natively exports
// both PLUGIN_ROOT and CLAUDE_PLUGIN_ROOT to plugin-bundled hooks for
// compatibility, so neither variable discriminates between hosts.
func DefaultRegistry() Registry {
	return Registry{
		{Platform: PlatformCodex, PayloadSniff: sniffCodexPayload},
		{Platform: PlatformClaude, PayloadSniff: sniffClaudePayload},
	}
}

// sniffCodexPayload matches the Codex hook wire shape: every verified Codex
// hook payload carries a top-level turn_id, which Claude payloads lack.
func sniffCodexPayload(top map[string]any) bool {
	_, ok := top["turn_id"]
	return ok
}

// sniffClaudePayload matches the Claude hook wire shape: a top-level
// hook_event_name without the Codex-only turn_id.
func sniffClaudePayload(top map[string]any) bool {
	if _, codex := top["turn_id"]; codex {
		return false
	}
	_, ok := top["hook_event_name"]
	return ok
}
