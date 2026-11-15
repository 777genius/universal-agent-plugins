package claude

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestClaudeMCPApprovalAddsOnlyExactSelectorAndPreservesDenial(t *testing.T) {
	selector := "plugin:demo:local"
	original := []byte(`{"permissions":{"deny":["Bash"],"allow":[]},"unrelated":9007199254740993,"enabledMcpjsonServers":["other"],"disabledMcpjsonServers":["blocked"]}`)
	next, err := renderClaudeMCPApproval(original, selector)
	if err != nil {
		t.Fatal(err)
	}
	for _, keep := range []string{`9007199254740993`, `"blocked"`, `"Bash"`, `"other"`, `"allow": []`} {
		if !bytes.Contains(next, []byte(keep)) {
			t.Fatalf("unrelated native setting lost: %s", next)
		}
	}
	var doc map[string]any
	if err := json.Unmarshal(next, &doc); err != nil {
		t.Fatal(err)
	}
	choices := doc["enabledMcpjsonServers"].([]any)
	if len(choices) != 2 || choices[1] != selector {
		t.Fatalf("approval not exact: %s", next)
	}
	again, err := renderClaudeMCPApproval(next, selector)
	if err != nil || !bytes.Equal(next, again) {
		t.Fatalf("approval not idempotent: %v", err)
	}
	denied := []byte(`{"disabledMcpjsonServers":["plugin:demo:local"]}`)
	kept, err := renderClaudeMCPApproval(denied, selector)
	if err != nil || !bytes.Equal(kept, denied) {
		t.Fatal("explicit rejection was overwritten")
	}
	for _, bad := range []string{`{"enabledMcpjsonServers":false}`, `{"disabledMcpjsonServers":[false]}`, `{"permissions":{},"permissions":{}}`, `null`, `{} {}`} {
		if _, err := renderClaudeMCPApproval([]byte(bad), selector); err == nil {
			t.Fatalf("malformed settings accepted: %s", bad)
		}
	}
}

func TestClaudeMCPHealthRequiresExactNativeSelectorAndSingleStatus(t *testing.T) {
	prefix := "plugin:demo:local:\n  Scope: Dynamic config (from command line)\n"
	for _, status := range []string{"✔ Connected", "✘ Failed to connect", "⏸ Pending approval (run `claude` to approve)", "✘ Rejected (see disabledMcpjsonServers in settings)"} {
		body := prefix + "  Status: " + status + "\n  Type: stdio\n  Command: stdio\n"
		got := observeClaudeMCPHealth([]byte(body), "plugin:demo:local")
		want := claudeMCPHealthUnready
		if status == "✔ Connected" {
			want = claudeMCPHealthConnected
		}
		if got.state != want {
			t.Fatalf("native status %q=%v", status, got)
		}
		for _, bad := range []string{strings.Replace(body, "plugin:demo:local:", "plugin:foreign:local:", 1), body + "  Status: ✔ Connected\n", strings.Replace(body, "Dynamic config (from command line)", "Local config", 1), "Description: ✔ Connected\n", body + "\x1b[0m"} {
			if observeClaudeMCPHealth([]byte(bad), "plugin:demo:local").state != claudeMCPHealthUnknown {
				t.Fatalf("unbound native output accepted: %q", bad)
			}
		}
	}
}

func TestClaudeMCPNamespaceRejectsSameNameAcrossNativeSources(t *testing.T) {
	for _, test := range []struct {
		body   string
		unique bool
	}{
		{`[{"id":"demo@skills-dir","enabled":true},{"id":"other@market","enabled":true}]`, true},
		{`[{"id":"demo@skills-dir","enabled":true},{"id":"demo@foreign-market","enabled":true}]`, false},
		{`[{"id":"demo@skills-dir","enabled":true},{"id":"demo@foreign-market","enabled":false}]`, true},
		{`[{"id":"demo@skills-dir","enabled":false}]`, false},
	} {
		got, err := claudeMCPNamespaceUnique([]byte(test.body), "demo")
		if err != nil || got != test.unique {
			t.Fatalf("namespace=%v err=%v body=%s", got, err, test.body)
		}
	}
	if _, err := claudeMCPNamespaceUnique([]byte(`[{"id":"demo@skills-dir","enabled":true,"enabled":false}]`), "demo"); err == nil {
		t.Fatal("ambiguous namespace keys accepted")
	}
}
