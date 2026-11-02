//go:build linux

package pluginkitairepo_test

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"path/filepath"
	"reflect"
)

const copilotReadinessDisabled = "Disabled tools: bash, create, dynamic_workflows_manage, edit, fetch_copilot_cli_documentation, glob, grep, list_agents, list_bash, read_agent, read_bash, run_dynamic_workflow, session_store_sql, skill, sql, stop_bash, task, view, write_agent"

func copilotReadinessValues(row map[string]any, expected map[string]any) bool {
	for key, value := range expected {
		if !reflect.DeepEqual(row[key], value) {
			return false
		}
	}
	return true
}
func (s *copilotReadinessStream) metadata(f *copilotProjectAttempt) error {
	for i, expected := range []map[string]any{
		{"serverName": "uapcopilota", "status": "pending"},
		{"serverName": "uapcopilota", "status": "connected", "configSource": "workspace"},
		{"serverName": "uapcopilota", "toolCount": json.Number("1")},
	} {
		if !copilotReadinessValues(s.data[i], expected) {
			return fmt.Errorf("Copilot readiness workspace server startup")
		}
	}
	if !copilotReadinessValues(s.data[4], map[string]any{"infoType": "configuration", "message": copilotReadinessDisabled}) || s.data[6]["model"] != "uap-fixture-model" {
		return fmt.Errorf("Copilot readiness selected tool/model configuration")
	}
	skills, ok := s.data[3]["skills"].([]any)
	if !ok || len(skills) != 3 {
		return fmt.Errorf("Copilot readiness exact skill catalog")
	}
	names := []string{"uapcopilota", "customize-cloud-agent", "github-pr-media"}
	descriptionHashes := []string{"", "b96d547993e3bf5142f5c01773688c2cad8c12a4cfddd3ca54cddf43838b101f", "6911e8b48dad09cfe8ed00b26792dc96939bc2ec9ab7b523d1d41e1109d8c444"}
	for i, value := range skills {
		row, err := copilotReadinessObject(value, "name commandName description source userInvocable enabled path")
		if err != nil {
			return err
		}
		source, path := "project", filepath.Join(f.base.Project, ".github", "skills", names[i], "SKILL.md")
		if i > 0 {
			source, path = "builtin", filepath.Join(f.base.Root, "xdg-cache", "copilot", "pkg", "linux-x64", "1.0.103", "builtin", names[i], "SKILL.md")
		}
		if !copilotReadinessValues(row, map[string]any{"name": names[i], "commandName": names[i], "source": source, "userInvocable": i == 0, "enabled": true, "path": path}) {
			return fmt.Errorf("Copilot readiness skill identity/authority")
		}
		description, ok := row["description"].(string)
		if !ok || i == 0 && description != "Disposable Copilot project proof" {
			return fmt.Errorf("Copilot readiness owned skill description")
		}
		if i > 0 {
			h := sha256.Sum256([]byte(description))
			if hex.EncodeToString(h[:]) != descriptionHashes[i] {
				return fmt.Errorf("Copilot readiness known builtin metadata")
			}
		}
	}
	servers, ok := s.data[5]["servers"].([]any)
	if !ok || len(servers) != 6 {
		return fmt.Errorf("Copilot readiness exact server catalog")
	}
	for i, name := range []string{"WorkIQ", "browser-use", "github-copilot-browser", "github-mcp-server", "githubiq", "uapcopilota"} {
		want := map[string]any{"name": name, "status": "disabled"}
		if i == 5 {
			want = map[string]any{"name": name, "status": "connected", "source": "workspace", "serverMetadata": map[string]any{"instructions": nil}}
		}
		if !reflect.DeepEqual(servers[i], want) {
			return fmt.Errorf("Copilot readiness server catalog authority")
		}
	}
	return nil
}
func (s *copilotReadinessStream) telemetry() error {
	for _, i := range []int{11, 21} {
		row := s.data[i]
		if !copilotReadinessNatural(row["dispatchDurationMs"], 60000) || row["outcome"] != "success" || row["editClassifierVersion"] != json.Number("1") || row["containsBuiltInFileEditRequest"] != false {
			return fmt.Errorf("Copilot readiness call telemetry")
		}
	}
	for _, i := range []int{14, 23} {
		if !copilotReadinessValues(s.data[i], map[string]any{"model": "uap-fixture-model", "isByok": true, "result": "success"}) {
			return fmt.Errorf("Copilot readiness scripted model outcome")
		}
	}
	telemetry, err := copilotReadinessObject(s.data[15]["toolTelemetry"], "metrics")
	if err != nil {
		return err
	}
	metrics, err := copilotReadinessObject(telemetry["metrics"], "mcp_result_content_bytes context_token_limit context_current_tokens large_output_threshold_bytes result_token_count")
	if err != nil {
		return err
	}
	for _, value := range metrics {
		if !copilotReadinessNatural(value, 1<<31) {
			return fmt.Errorf("Copilot readiness bounded tool metrics")
		}
	}
	usage, err := copilotReadinessObject(s.rows[26]["usage"], "premiumRequests totalApiDurationMs sessionDurationMs codeChanges")
	if err != nil || usage["premiumRequests"] != json.Number("0") || !copilotReadinessNatural(usage["totalApiDurationMs"], 60000) || !copilotReadinessNatural(usage["sessionDurationMs"], 60000) || !reflect.DeepEqual(usage["codeChanges"], map[string]any{"linesAdded": json.Number("0"), "linesRemoved": json.Number("0"), "filesModified": []any{}}) {
		return fmt.Errorf("Copilot readiness terminal telemetry/code changes")
	}
	return nil
}
