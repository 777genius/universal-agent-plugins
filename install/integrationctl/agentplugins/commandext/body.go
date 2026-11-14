package commandext

import (
	"bytes"
	"regexp"
	"unicode/utf8"
)

// Body rules run over the whole text, fenced code included: a client expands
// these forms wherever they appear, and a command must mean the same thing in
// every client it reaches.
var (
	pluginVariable = regexp.MustCompile(`\$\{?(?:CLAUDE_PLUGIN_ROOT|CLAUDE_PLUGIN_DATA|CLAUDE_SKILL_DIR|COPILOT_PLUGIN_ROOT|CURSOR_PLUGIN_ROOT|DEVIN_PLUGIN_ROOT|PLUGIN_ROOT|PLUGIN_DATA)\b|\$\{extensionPath\}`)
	clientVariable = regexp.MustCompile(`\$\{(?:CLAUDE|COPILOT|CURSOR|DEVIN)_[A-Z0-9_]+\}`)
	argument       = regexp.MustCompile(`\$\{?(?:ARGUMENTS|\d)|\{\{\s*args\s*\}\}`)
	// Claude Code runs !`cmd` and blocks fenced with ```! in a skill before
	// the model reads it; Gemini CLI runs !{cmd} in its commands. A fence is
	// matched anywhere, not only at the start of a line: Markdown also opens
	// one inside a list item or a quote, and the check must not depend on
	// how a client reads line starts.
	shellInjection = regexp.MustCompile("!`|!\\{|(?:`{3,}|~{3,})!")
)

func checkBody(body []byte) []finding {
	var findings []finding
	if !utf8.Valid(body) || bytes.IndexByte(body, 0) >= 0 {
		findings = append(findings, finding{"command_body_encoding", "the command body must be UTF-8 text without NUL bytes"})
	}
	if len(bytes.TrimSpace(body)) == 0 {
		findings = append(findings, finding{"command_body_empty", "the command body is empty"})
	}
	if pluginVariable.Match(body) {
		findings = append(findings, finding{"command_plugin_variable", "the body uses a plugin root or data variable, which differs between clients"})
	}
	if hasClientVariable(body) {
		findings = append(findings, finding{"command_client_variable", "the body uses a client-specific variable"})
	}
	if argument.Match(body) {
		findings = append(findings, finding{"command_argument_placeholder", "the body uses an argument placeholder, which clients expand differently"})
	}
	if shellInjection.Match(body) {
		findings = append(findings, finding{"command_shell_injection", "the body uses inline shell execution syntax"})
	}
	return findings
}

// hasClientVariable ignores a ${...} that pluginVariable already reports, so
// ${CLAUDE_PLUGIN_ROOT} yields one diagnostic rather than two.
func hasClientVariable(body []byte) bool {
	reported := map[int]bool{}
	for _, match := range pluginVariable.FindAllIndex(body, -1) {
		reported[match[0]] = true
	}
	for _, match := range clientVariable.FindAllIndex(body, -1) {
		if !reported[match[0]] {
			return true
		}
	}
	return false
}
