package agentpluginscli

import (
	"bytes"
	"encoding/json"
	"errors"
	"testing"

	"github.com/spf13/cobra"
	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli/skillspicker"
)

func TestAgentSkillsSuccessAndPartialMachineOutcomes(t *testing.T) {
	for _, test := range []struct{ status, result, readiness string }{
		{"succeeded", "success", "needs_action"},
		{"partial", "failure", "failed"},
	} {
		t.Run(test.status, func(t *testing.T) {
			var output bytes.Buffer
			cmd := &cobra.Command{Use: "skills"}
			cmd.SetOut(&output)
			if err := renderSkillsResult(cmd, &options{format: "json", scope: "user"}, SkillsBatchResult{Status: test.status}); err != nil {
				t.Fatal(err)
			}
			var doc struct {
				SchemaVersion int    `json:"schema_version"`
				Command       string `json:"command"`
				Result        string `json:"result"`
				Status        string `json:"status"`
				Readiness     string `json:"readiness"`
			}
			if err := json.Unmarshal(output.Bytes(), &doc); err != nil {
				t.Fatal(err)
			}
			if doc.SchemaVersion != 1 || doc.Command != "skills add" || doc.Result != test.result ||
				doc.Readiness != test.readiness || doc.Status != test.status {
				t.Fatalf("wrong Skills machine result: %s", output.String())
			}
		})
	}
}

func TestSkillsDryRunKeepsLegacyFieldsAndAddsOutcome(t *testing.T) {
	var buf bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&buf)
	if err := renderSkillsDryRun(cmd, skillspicker.Plan{}); err != nil {
		t.Fatal(err)
	}
	var raw map[string]any
	if err := json.Unmarshal(buf.Bytes(), &raw); err != nil {
		t.Fatal(err)
	}
	if raw["status"] != "planned" || raw["dryRun"] != true || raw["schema_version"] != float64(1) ||
		raw["command"] != "skills add" || raw["result"] != "success" {
		t.Fatalf("missing compatible Skills preview fields: %s", buf.String())
	}
}

func TestAgentYesIsScopedToMutatingCommands(t *testing.T) {
	root := NewRoot(App{})
	add, _, err := root.Find([]string{"add"})
	if err != nil || add.Flags().Lookup("yes") == nil {
		t.Fatalf("package add does not accept explicit --yes: %v", err)
	}
	install, _, err := root.Find([]string{"install"})
	if err != nil || install.Flags().Lookup("yes") == nil {
		t.Fatalf("install does not accept explicit --yes: %v", err)
	}
	for _, name := range []string{"update", "repair", "remove"} {
		command, _, err := root.Find([]string{name})
		if err != nil || command.Flags().Lookup("yes") == nil {
			t.Fatalf("%s does not accept explicit --yes: %v", name, err)
		}
	}
	list, _, err := root.Find([]string{"list"})
	if err != nil || list.Flags().Lookup("yes") != nil {
		t.Fatalf("read-only list must not accept --yes: %v", err)
	}
}

func TestAgentExplicitYesNeverStartsAmbiguousTargetSelection(t *testing.T) {
	for _, argv := range [][]string{
		{"add", "context7", "--yes"},
		{"remove", "demo", "--yes"},
	} {
		var output bytes.Buffer
		cmd := NewRoot(App{Terminal: true, Output: &output, ErrorOutput: &output})
		cmd.SetArgs(argv)
		err := cmd.Execute()
		var usage CLIUsageError
		if !errors.As(err, &usage) || usage.Code != "TARGET_REQUIRED" {
			t.Fatalf("%v: expected fail-closed target selection before acquisition, got %v; output %q", argv, err, output.String())
		}
		if output.Len() != 0 {
			t.Fatalf("%v: selection unexpectedly displayed a prompt: %s", argv, output.String())
		}
	}
}
