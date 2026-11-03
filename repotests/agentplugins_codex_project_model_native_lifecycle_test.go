package pluginkitairepo_test

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestAgentpluginsCodexNativeProjectModelForeignMCP(t *testing.T) {
	if os.Getenv("AGENTPLUGINS_CODEX_PROJECT_NATIVE_E2E") != "1" {
		t.Skip("source-only pinned Codex project model tool-call test")
	}
	nativeRequireDisposable(t)
	if _, err := os.Lstat("/etc/codex"); !os.IsNotExist(err) {
		t.Fatal("disposable host must have no system Codex config")
	}
	f := &codexProjectNativeFixture{base: newNativeFixture(t, true),
		client:    nativeBinary(t, "AGENTPLUGINS_CODEX_BIN"),
		installer: nativeBinary(t, "AGENTPLUGINS_INSTALLER_BIN"),
		unchanged: map[string]string{},
	}
	if want := os.Getenv("AGENTPLUGINS_CODEX_SHA256"); len(want) != 64 || nativeSHA(t, f.client) != want {
		t.Fatal("pinned native Codex binary mismatch")
	}
	f.base.ClientBinDir, f.otherProject = filepath.Dir(f.client), filepath.Join(f.base.Root, "project-B")
	for _, project := range []string{f.base.Project, f.otherProject} {
		if err := os.MkdirAll(filepath.Join(project, ".git"), 0700); err != nil {
			t.Fatal(err)
		}
	}
	model := newCodexProjectModel(t)
	config := fmt.Sprintf("cli_auth_credentials_store = \"file\"\nmcp_oauth_credentials_store = \"file\"\nmodel_provider = \"fixture\"\nmodel = \"fixture-model\"\n[model_providers.fixture]\nname = \"Fixture\"\nbase_url = %q\nwire_api = \"responses\"\nrequires_openai_auth = false\n[analytics]\nenabled = false\n[feedback]\nenabled = false\n[projects.%q]\ntrust_level = \"trusted\"\n[projects.%q]\ntrust_level = \"trusted\"\n", model.server.URL+"/v1", f.base.Project, f.otherProject)
	f.foreign(t, filepath.Join(f.base.CodexHome, "config.toml"), config)
	f.foreign(t, filepath.Join(f.otherProject, ".codex", "config.toml"), "# project B is foreign\nmodel = 'fixture-model'\n")
	f.foreign(t, filepath.Join(f.base.Project, "foreign-keep.txt"), "keep foreign A bytes\n")
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	events := filepath.Join(f.base.Root, "codex-foreign-mcp-events.log")
	foreign := fmt.Sprintf("# foreign MCP belongs to the user, not UAP\n[mcp_servers.foreign_fixture]\ncommand = %q\nargs = [\"-test.run=^TestOpenCodeV2MCPStdioResponder$\"]\n[mcp_servers.foreign_fixture.env]\nUAP_OPENCODE_V2_MCP_HELPER = \"1\"\nUAP_OPENCODE_V2_MCP_EVENTS = %q\n", executable, events)
	projectConfig := filepath.Join(f.base.Project, ".codex", "config.toml")
	nativeWrite(t, projectConfig, []byte(foreign), 0600)
	f.base.writePackage(t, "A", "1.0.0")

	stages := map[string]nativeStage{
		"install_preserved_foreign": {Status: "not_evaluated", Reason: "not reached"},
		"model_native_tool_before":  {Status: "not_evaluated", Reason: "not reached"},
		"project_B_no_leak":         {Status: "not_evaluated", Reason: "not reached"},
		"remove_foreign_retention":  {Status: "not_evaluated", Reason: "not reached"},
		"model_native_tool_after":   {Status: "not_evaluated", Reason: "not reached"},
	}
	evidence := f.evidence(t, stages)
	evidence["project_contract"] = "codex_trusted_foreign_project_model_mcp_v1"
	evidence["native_surface"] = "real pinned Codex model-driven foreign stdio MCP; scripted local Responses model, no vendor authentication"
	defer func() {
		f.writeEvidence(t, evidence)
		if err := nativeStageFailures(stages); err != nil {
			t.Error(err)
		}
	}()
	added := f.run(t, "codex-model-foreign-add", "add", f.base.Package, "--no-save")
	codexNativeProjectPrepared(t, added)
	codexProjectMCPBinding(t, f)
	codexAssertForeignProjectMCP(t, projectConfig, foreign, true)
	stages["install_preserved_foreign"] = nativeStage{Status: "passed", Reason: "project add retained foreign project MCP alongside UAP-managed MCP, with user/B isolation"}

	codexProjectModelSession(t, f, model, f.base.Project, "codex-model-before-remove-A", true, true)
	beforeCalls, beforeB := codexProjectMCPEventCount(t, events)
	if beforeCalls != 1 {
		t.Fatalf("real Codex before-remove MCP tools/call count=%d", beforeCalls)
	}
	stages["model_native_tool_before"] = nativeStage{Status: "passed", Reason: "native model turn chose exact foreign MCP tool; real child called and uap-pong returned to local Responses provider"}
	codexProjectModelSession(t, f, model, f.otherProject, "codex-model-control-B", false, false)
	afterB, err := os.ReadFile(events)
	if err != nil || !bytes.Equal(afterB, beforeB) {
		t.Fatal("control project B connected or executed project A foreign MCP")
	}
	stages["project_B_no_leak"] = nativeStage{Status: "passed", Reason: "trusted B model turn did not discover or execute foreign project A MCP"}
	f.run(t, "codex-model-foreign-remove", "remove", "native-proof")
	codexAssertForeignProjectMCP(t, projectConfig, foreign, false)
	f.requireForeign(t)
	stages["remove_foreign_retention"] = nativeStage{Status: "passed", Reason: "remove preserved exact foreign TOML values and original comment while removing only owned entries; user and B unchanged"}

	codexProjectModelSession(t, f, model, f.base.Project, "codex-model-after-remove-A", true, false)
	afterCalls, _ := codexProjectMCPEventCount(t, events)
	if afterCalls != 2 {
		t.Fatalf("foreign MCP stopped working after UAP removal: tools/call=%d", afterCalls)
	}
	stages["model_native_tool_after"] = nativeStage{Status: "passed", Reason: "fresh native Codex model turn called untouched foreign MCP after UAP removal and returned uap-pong"}
}
