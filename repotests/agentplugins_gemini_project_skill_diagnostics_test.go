//go:build linux

package pluginkitairepo_test

import (
	"strings"
	"testing"
)

const geminiPureSkillUntrustedStderr = "Skipping project agents due to untrusted folder. To enable, ensure that the project root is trusted.\n" +
	"Ripgrep is not available. Falling back to GrepTool.\n" +
	"Project hooks disabled because the folder is not trusted.\n"

func geminiPureSkillListingStderr(body []byte, phase, scope string) bool {
	switch phase {
	case "baseline", "installed", "repeat", "removed":
	default:
		return false
	}
	switch scope {
	case "a", "b", "user":
	default:
		return false
	}
	// B starts trusted. Confirmed installation grants A trust, which removal
	// preserves. The distinct outside/User directory never receives trust.
	untrusted := scope == "user" || scope == "a" && phase == "baseline"
	if untrusted {
		return string(body) == geminiPureSkillUntrustedStderr
	}
	return len(body) == 0 || string(body) == "Ripgrep is not available. Falling back to GrepTool.\n"
}

func TestGeminiPureSkillListingStderrRequiresExpectedTrustState(t *testing.T) {
	const ripgrep = "Ripgrep is not available. Falling back to GrepTool.\n"
	measured := []byte(geminiPureSkillUntrustedStderr)
	if len(measured) != 211 || geminiCycleSHA(measured) != "90f97337101f629e2177832ecdca61c30015861ed2358dffeceb640a894d917b" {
		t.Fatal("retained genuine untrusted baseline diagnostics differ")
	}
	for _, state := range []struct {
		phase, scope string
		untrusted    bool
	}{
		{"baseline", "a", true}, {"baseline", "b", false}, {"baseline", "user", true},
		{"installed", "a", false}, {"installed", "b", false}, {"installed", "user", true},
		{"repeat", "a", false}, {"repeat", "b", false}, {"repeat", "user", true},
		{"removed", "a", false}, {"removed", "b", false}, {"removed", "user", true},
	} {
		for _, body := range []string{"", ripgrep, geminiPureSkillUntrustedStderr} {
			want := body == geminiPureSkillUntrustedStderr
			if !state.untrusted {
				want = !want
			}
			if geminiPureSkillListingStderr([]byte(body), state.phase, state.scope) != want {
				t.Fatal("pure Skill diagnostics accepted the wrong trust state", state.phase, state.scope, body)
			}
		}
	}
}

func TestGeminiPureSkillListingStderrRejectsIncompleteOrUnknownDiagnostics(t *testing.T) {
	body := geminiPureSkillUntrustedStderr
	lines := strings.SplitAfter(body, "\n")
	for _, altered := range []string{
		body[:len(body)-1], body + "\n", "\n" + body, body + body,
		strings.ReplaceAll(body, "\n", "\r\n"), "\x1b[31m" + body, body + "\x00",
		lines[0] + lines[1], lines[1] + lines[2], lines[0] + lines[2],
		lines[0] + lines[0] + lines[1] + lines[2], lines[2] + lines[1] + lines[0],
		body + "Authentication required\n", body + "Scheduling MCP context refresh...\n",
	} {
		for _, scope := range []string{"a", "user"} {
			if geminiPureSkillListingStderr([]byte(altered), "baseline", scope) {
				t.Fatal("pure Skill diagnostics admitted missing, duplicate or unknown output", scope, altered)
			}
		}
	}
	for _, state := range [][2]string{{"unknown", "a"}, {"baseline", "unknown"}, {"", "user"}} {
		for _, candidate := range []string{"", body} {
			if geminiPureSkillListingStderr([]byte(candidate), state[0], state[1]) {
				t.Fatal("unknown pure Skill phase or scope accepted")
			}
		}
	}
	if geminiCycleListingStderr([]byte(body), "a", nil) {
		t.Fatal("pure Skill diagnostics widened the mixed MCP inventory contract")
	}
}
