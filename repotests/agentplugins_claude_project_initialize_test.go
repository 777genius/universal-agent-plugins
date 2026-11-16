package pluginkitairepo_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
)

// Initialize sends no user message. The genuine CLI reports the Skill commands
// it loaded, then closes naturally on EOF with no account token or model call.
func claudeProjectInitializedSkills(t *testing.T, f *claudeProjectNativeFixture, label string, expected []string, present bool) {
	t.Helper()
	args := []string{"-p", "--input-format", "stream-json", "--output-format", "stream-json", "--verbose", "--permission-mode", "dontAsk", "--strict-mcp-config", "--mcp-config", `{"mcpServers":{}}`}
	input := []byte("{\"type\":\"control_request\",\"request_id\":\"native-discovery\",\"request\":{\"subtype\":\"initialize\"}}\n")
	for _, selected := range []struct {
		suffix, project string
		active          bool
	}{{"A", f.project, present}, {"B", f.otherProject, false}} {
		result := captureClaudeDiagnosticInput(f.client, selected.project, f.base.env(filepath.Dir(f.client)), args, input)
		prefix := label + "-initialize-" + selected.suffix
		nativeWrite(t, filepath.Join(f.base.Root, prefix+".stdout.log"), result.Stdout, 0600)
		nativeWrite(t, filepath.Join(f.base.Root, prefix+".stderr.log"), result.Stderr, 0600)
		nativeJSON(t, filepath.Join(f.base.Root, prefix+".json"), map[string]any{"argv": args, "cwd": selected.project,
			"exit": result.Exit, "error": result.Error, "deadline": result.Deadline, "stdout_limit": result.StdoutLimit, "stderr_limit": result.StderrLimit,
			"request": json.RawMessage(bytes.TrimSpace(input)), "model_request": false})
		if result.Exit != 0 || result.Error != "" || result.Deadline || result.StdoutLimit || result.StderrLimit {
			t.Fatal("credential-free native initialization failed; raw evidence retained")
		}
		if err := validateClaudeInitializedSkills(result.Stdout, expected, selected.active); err != nil {
			t.Fatal(err)
		}
	}
	f.requireForeign(t)
}

func validateClaudeInitializedSkills(body []byte, expected []string, present bool) error {
	value, err := shared.DecodeUniqueJSONValue(json.NewDecoder(bytes.NewReader(body)))
	message, ok := value.(map[string]any)
	if err != nil || !json.Valid(body) || !ok || message["type"] != "control_response" {
		return fmt.Errorf("native initialization did not return one unique control response")
	}
	response, _ := message["response"].(map[string]any)
	if response["request_id"] != "native-discovery" || response["subtype"] != "success" {
		return fmt.Errorf("native initialization response has the wrong request or failed")
	}
	result, _ := response["response"].(map[string]any)
	account, _ := result["account"].(map[string]any)
	if account["tokenSource"] != "none" {
		return fmt.Errorf("native initialization used or could not exclude account credentials")
	}
	commands, ok := result["commands"].([]any)
	if !ok {
		return fmt.Errorf("native initialized Skill inventory missing")
	}
	return validateClaudeInitializedCommands(commands, expected, present)
}

func validateClaudeInitializedCommands(commands []any, expected []string, present bool) error {
	wanted, prefixes, found := map[string]bool{}, map[string]bool{}, map[string]bool{}
	for _, name := range expected {
		wanted[name], prefixes[strings.SplitN(name, ":", 2)[0]+":"] = true, true
	}
	for _, item := range commands {
		command, ok := item.(map[string]any)
		name, named := command["name"].(string)
		if !ok || !named {
			return fmt.Errorf("native initialized command is malformed")
		}
		if !prefixes[strings.SplitN(name, ":", 2)[0]+":"] {
			continue
		}
		if !present || !wanted[name] || found[name] || command["builtin"] == true {
			return fmt.Errorf("native initialized Skill is unexpected, duplicated, or active in the wrong project")
		}
		found[name] = true
	}
	if present && len(found) != len(wanted) {
		return fmt.Errorf("native initialization did not load every selected Skill")
	}
	return nil
}
