package platformexec

import (
	"encoding/json"
	"fmt"
	"strings"
)

// qoderHookNames are the Claude-derived hook events the Qoder CLI accepts and
// migrates (qoder hooks migrate). Shipped Qoder plugins exercise SessionStart,
// UserPromptSubmit, PostToolUse and Stop.
func qoderHookNames() []string {
	return []string{
		"SessionStart", "SessionEnd", "Notification", "PostToolUse", "PostToolUseFailure",
		"PermissionRequest", "SubagentStart", "SubagentStop", "PreCompact", "Setup",
		"Stop", "PreToolUse", "TeammateIdle", "TaskCompleted", "UserPromptSubmit",
		"ConfigChange", "WorktreeCreate", "WorktreeRemove",
	}
}

// qoderHookRootToken is the only path envelope a bundled hook command may use:
// the Qoder runtime resolves it to the installed plugin root, so an authored
// command escaping the bundle cannot be projected safely.
const qoderHookRootToken = "${QODER_PLUGIN_ROOT}"

type qoderHooksDocument struct {
	Description string                      `json:"description,omitempty"`
	Hooks       map[string][]qoderHookGroup `json:"hooks"`
}

type qoderHookGroup struct {
	Matcher string          `json:"matcher,omitempty"`
	Async   *bool           `json:"async,omitempty"`
	Hooks   []qoderHookSpec `json:"hooks"`
}

type qoderHookSpec struct {
	Type    string            `json:"type"`
	Command string            `json:"command"`
	Args    []string          `json:"args,omitempty"`
	Name    string            `json:"name,omitempty"`
	Timeout *float64          `json:"timeout,omitempty"`
	Env     map[string]string `json:"env,omitempty"`
}

func parseQoderHooksDocument(body []byte) (qoderHooksDocument, error) {
	var doc qoderHooksDocument
	if err := json.Unmarshal(body, &doc); err != nil {
		return qoderHooksDocument{}, fmt.Errorf("parse hooks document: %w", err)
	}
	if doc.Hooks == nil {
		return qoderHooksDocument{}, fmt.Errorf(`hooks document must define a "hooks" object`)
	}
	return doc, nil
}

func qoderHookNameRecognized(name string) bool {
	for _, known := range qoderHookNames() {
		if known == name {
			return true
		}
	}
	return false
}

// qoderHookCommandBundled reports whether command stays inside the managed
// plugin tree: a bare runtime name ("node") or the ${QODER_PLUGIN_ROOT} token.
// Absolute host paths would silently escape the managed bundle.
func qoderHookCommandBundled(command string) bool {
	command = strings.TrimSpace(command)
	if command == "" {
		return false
	}
	if strings.ContainsAny(command, "/\\") {
		return strings.HasPrefix(command, qoderHookRootToken+"/")
	}
	return true
}
