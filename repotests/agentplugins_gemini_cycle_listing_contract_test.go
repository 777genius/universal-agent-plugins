//go:build linux

package pluginkitairepo_test

import (
	"strings"
	"testing"
)

const geminiCycleMeasuredUserListingStderr = `Ripgrep is not available. Falling back to GrepTool.
Registering notification handlers for server 'uapgeminiu'. Capabilities: { tools: {} }
Server 'uapgeminiu' has tools but did not declare 'listChanged' capability. Listening anyway for robustness...
Scheduling MCP context refresh...
Executing MCP context refresh...
MCP context refresh complete.
`

func geminiCycleDiagnosticRecipe(scope string) *geminiCycleRecipe {
	r := &geminiCycleRecipe{Scope: scope, Name: map[string]string{"user": "uapgeminiu", "a": "uapgeminia", "b": "uapgeminib"}[scope]}
	r.Binding.ClientBindingID = "inert-held-" + scope
	return r
}

func TestGeminiCycleListingStderrPinsWholeMeasuredTupleAndFiniteScopedPermutations(t *testing.T) {
	body := []byte(geminiCycleMeasuredUserListingStderr)
	if len(body) != 347 || geminiCycleSHA(body) != "76222dc470d4288bb95b6a1c18044afad792af771c28d2018ecaa43b2d64e5a0" {
		t.Fatal("retained genuine listing stderr byte/hash golden differs")
	}
	lines := strings.SplitAfter(string(body), "\n")
	pair := lines[1] + lines[2]
	prefix, suffix := lines[0], strings.Join(lines[3:], "")
	for _, scope := range []string{"user", "a", "b"} {
		r := geminiCycleDiagnosticRecipe(scope)
		owned := strings.ReplaceAll(pair, "uapgeminiu", r.Name)
		if !geminiCycleListingStderr([]byte(prefix+owned+suffix), scope, []*geminiCycleRecipe{r}) {
			t.Fatal("finite selected-scope diagnostic refused", scope)
		}
		if scope != "user" {
			for _, text := range []string{prefix + pair + owned + suffix, prefix + owned + pair + suffix} {
				if !geminiCycleListingStderr([]byte(text), scope, []*geminiCycleRecipe{geminiCycleDiagnosticRecipe("user"), r}) {
					t.Fatal("concurrent discovery pair permutation refused", scope)
				}
			}
		}
		for _, legacy := range []string{"", prefix, prefix + suffix} {
			if !geminiCycleListingStderr([]byte(legacy), scope, nil) || !geminiCycleListingStderr([]byte(legacy), scope, []*geminiCycleRecipe{r}) {
				t.Fatal("legacy exact tuple changed")
			}
		}
	}
}

func TestGeminiCycleListingStderrRefusesPartialUnknownOrChangedDiagnostics(t *testing.T) {
	body := geminiCycleMeasuredUserListingStderr
	lines := strings.SplitAfter(body, "\n")
	for index, altered := range []string{
		body[:len(body)-1], body + "\n", "\n" + body, body + body,
		strings.ReplaceAll(body, "\n", "\r\n"), "\x1b[31m" + body, body + "\x00",
		lines[0] + lines[1] + strings.Join(lines[3:], ""), lines[0] + strings.Join(lines[2:], ""),
		lines[0] + lines[2] + lines[1] + strings.Join(lines[3:], ""),
		lines[0] + lines[1] + lines[2] + lines[1] + lines[2] + strings.Join(lines[3:], ""),
		strings.ReplaceAll(body, "uapgeminiu", "uapgeminia"), strings.ReplaceAll(body, "uapgeminiu", "foreign-user"),
		strings.Replace(body, "{ tools: {} }", "{ tools: { listChanged: true } }", 1),
		strings.Replace(body, "has tools", "has resources", 1), strings.Replace(body, "Capabilities:", "Capabilities", 1),
		strings.Replace(body, "Listening anyway", "Not listening", 1),
		body + "API request failed\n", body + "Authentication required\n", "Error: private stack\n" + body,
		body + strings.Join(lines[3:], ""), strings.Repeat("x", 1025),
	} {
		if geminiCycleListingStderr([]byte(altered), "user", []*geminiCycleRecipe{geminiCycleDiagnosticRecipe("user")}) {
			t.Fatal("unmeasured/failing listing diagnostic admitted", index)
		}
	}
	u, a := geminiCycleDiagnosticRecipe("user"), geminiCycleDiagnosticRecipe("a")
	aLine := strings.ReplaceAll(lines[1], u.Name, a.Name)
	aWarning := strings.ReplaceAll(lines[2], u.Name, a.Name)
	for _, altered := range []string{
		lines[0] + lines[1] + aLine + lines[2] + aWarning + strings.Join(lines[3:], ""),
		body, strings.ReplaceAll(lines[0]+lines[1]+lines[2]+aLine+aWarning+strings.Join(lines[3:], ""), a.Name, "uapgeminib"),
	} {
		if geminiCycleListingStderr([]byte(altered), "a", []*geminiCycleRecipe{u, a}) {
			t.Fatal("partial/interleaved/other-project name set accepted")
		}
	}
}

func TestGeminiCycleListingStderrCannotDeriveAuthorityFromInvalidRecipes(t *testing.T) {
	for _, fault := range []string{"nil", "duplicate", "unknown-scope", "unknown-name", "scope-name", "removed", "unproved", "too-many", "other-project", "two-projects", "invalid-listing-scope"} {
		t.Run(fault, func(t *testing.T) {
			r := geminiCycleDiagnosticRecipe("user")
			scope := "user"
			advertised := make([]*geminiCycleRecipe, 1, 4)
			advertised[0] = r
			switch fault {
			case "nil":
				advertised[0] = nil
			case "duplicate":
				advertised = append(advertised, r)
			case "unknown-scope":
				r.Scope = "foreign"
			case "unknown-name":
				r.Name = "uapgeminiu' private-injection"
			case "scope-name":
				r.Name = "uapgeminia"
			case "removed":
				r.Removed = true
			case "unproved":
				r.Binding.ClientBindingID = ""
			case "too-many":
				advertised = append(advertised, geminiCycleDiagnosticRecipe("a"), geminiCycleDiagnosticRecipe("b"))
			case "other-project":
				scope, advertised[0] = "a", geminiCycleDiagnosticRecipe("b")
			case "two-projects":
				scope, advertised[0] = "a", geminiCycleDiagnosticRecipe("a")
				advertised = append(advertised, geminiCycleDiagnosticRecipe("b"))
			case "invalid-listing-scope":
				scope = "foreign"
			}
			for _, body := range []string{"", geminiCycleMeasuredUserListingStderr} {
				if geminiCycleListingStderr([]byte(body), scope, advertised) {
					t.Fatal("invalid advertised authority created a diagnostic candidate")
				}
			}
		})
	}
}
