package platformexec

import (
	"fmt"
	"slices"
	"strings"
)

func inferGeminiEntrypoint(body []byte) (string, bool) {
	hooks, err := parseGeminiHooks(body)
	if err != nil {
		return "", false
	}
	for _, hookName := range stableGeminiHookNames() {
		invocation := geminiInvocationAlias(hookName)
		for _, entry := range hooks.Hooks[hookName] {
			for _, command := range entry.Hooks {
				if strings.TrimSpace(command.Type) != "command" {
					continue
				}
				if entrypoint, ok := trimGeminiHookCommand(command.Command, invocation); ok {
					return entrypoint, true
				}
			}
		}
	}
	return "", false
}

func defaultGeminiHooks(entrypoint string) []byte {
	type hookCommand struct {
		Name    string `json:"name,omitempty"`
		Type    string `json:"type"`
		Command string `json:"command"`
	}
	type hookEntry struct {
		Matcher string        `json:"matcher,omitempty"`
		Hooks   []hookCommand `json:"hooks"`
	}
	hooks := map[string][]hookEntry{}
	for _, name := range stableGeminiHookNames() {
		invocation := geminiInvocationAlias(name)
		commands := geminiHookCommandCandidates(entrypoint, invocation)
		command := strings.TrimSpace(entrypoint) + " " + invocation
		if len(commands) > 0 {
			command = commands[len(commands)-1]
		}
		hooks[name] = []hookEntry{{
			Matcher: "*",
			Hooks: []hookCommand{{
				Type:    "command",
				Command: command,
			}},
		}}
	}
	body, _ := marshalJSON(map[string]any{"hooks": hooks})
	return body
}

func validateGeminiHookEntrypoints(body []byte, entrypoint string) ([]string, error) {
	hooks, err := parseGeminiHooks(body)
	if err != nil {
		return nil, err
	}
	var mismatches []string
	for _, hookName := range stableGeminiHookNames() {
		matcher := "*"
		invocation := geminiInvocationAlias(hookName)
		entries := hooks.Hooks[hookName]
		expectedCommands := geminiHookCommandCandidates(entrypoint, invocation)
		expected := strings.TrimSpace(entrypoint) + " " + invocation
		if len(expectedCommands) > 0 {
			expected = expectedCommands[len(expectedCommands)-1]
		}
		if len(entries) == 0 {
			mismatches = append(mismatches, fmt.Sprintf("entrypoint mismatch: Gemini hook %q is missing; expected %q", hookName, expected))
			continue
		}
		foundCommand := false
		for _, entry := range entries {
			if strings.TrimSpace(entry.Matcher) != matcher {
				mismatches = append(mismatches, fmt.Sprintf("matcher mismatch: Gemini hook %q uses %q; expected %q", hookName, entry.Matcher, matcher))
			}
			for _, command := range entry.Hooks {
				if strings.TrimSpace(command.Type) != "command" {
					continue
				}
				foundCommand = true
				if !slices.Contains(expectedCommands, strings.TrimSpace(command.Command)) {
					mismatches = append(mismatches, fmt.Sprintf("entrypoint mismatch: Gemini hook %q uses %q; expected %q from launcher.yaml entrypoint", hookName, command.Command, expected))
				}
			}
		}
		if !foundCommand {
			mismatches = append(mismatches, fmt.Sprintf("entrypoint mismatch: Gemini hook %q declares no command hooks; expected %q", hookName, expected))
		}
	}
	return mismatches, nil
}
