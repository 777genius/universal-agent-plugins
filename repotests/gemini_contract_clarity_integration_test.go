package pluginkitairepo_test

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestContractClarity_GeminiRuntimeDocsStayAligned(t *testing.T) {
	root := RepoRoot(t)
	pluginKitAIBin := buildPluginKitAI(t)

	matrixBody, err := os.ReadFile(filepath.Join(root, "docs", "generated", "support_matrix.md"))
	if err != nil {
		t.Fatal(err)
	}
	matrix := string(matrixBody)
	for _, row := range []string{
		"| gemini | SessionStart | runtime_supported | stable | production-ready | true |",
		"| gemini | SessionEnd | runtime_supported | stable | production-ready | true |",
		"| gemini | BeforeModel | runtime_supported | stable | production-ready | true |",
		"| gemini | AfterModel | runtime_supported | stable | production-ready | true |",
		"| gemini | BeforeToolSelection | runtime_supported | stable | production-ready | true |",
		"| gemini | BeforeAgent | runtime_supported | stable | production-ready | true |",
		"| gemini | AfterAgent | runtime_supported | stable | production-ready | true |",
		"| gemini | BeforeTool | runtime_supported | stable | production-ready | true |",
		"| gemini | AfterTool | runtime_supported | stable | production-ready | true |",
	} {
		mustContain(t, matrix, row)
	}

	targetMatrixBody, err := os.ReadFile(filepath.Join(root, "docs", "generated", "target_support_matrix.md"))
	if err != nil {
		t.Fatal(err)
	}
	targetMatrix := string(targetMatrixBody)
	mustContain(t, targetMatrix, "| gemini | extension_package | mcp_extension | optional | extension | copy install | link | restart required | ~/.gemini/extensions/<name> | production-ready extension packaging lane |")
	mustContain(t, targetMatrix, "production-ready extension packaging plus optional production-ready 9-hook Go runtime")

	cmd := exec.Command(pluginKitAIBin, "capabilities", "--mode", "runtime", "--format", "json", "--platform", "gemini")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("capabilities json: %v\n%s", err, out)
	}
	var entries []map[string]any
	if err := json.Unmarshal(out, &entries); err != nil {
		t.Fatalf("parse capabilities json: %v\n%s", err, out)
	}
	byKey := map[string]map[string]any{}
	for _, entry := range entries {
		key := entry["platform"].(string) + "/" + entry["event"].(string)
		byKey[key] = entry
	}
	for _, key := range []string{
		"gemini/SessionStart",
		"gemini/SessionEnd",
		"gemini/BeforeModel",
		"gemini/AfterModel",
		"gemini/BeforeToolSelection",
		"gemini/BeforeAgent",
		"gemini/AfterAgent",
		"gemini/BeforeTool",
		"gemini/AfterTool",
	} {
		assertCapabilityContract(t, byKey, key, "stable", "production-ready")
	}

	productionDoc, err := os.ReadFile(filepath.Join(root, "docs", "PRODUCTION.md"))
	if err != nil {
		t.Fatal(err)
	}
	supportDoc, err := os.ReadFile(filepath.Join(root, "docs", "SUPPORT.md"))
	if err != nil {
		t.Fatal(err)
	}
	sdkReadme, err := os.ReadFile(filepath.Join(root, "sdk", "README.md"))
	if err != nil {
		t.Fatal(err)
	}
	sdkStability, err := os.ReadFile(filepath.Join(root, "sdk", "STABILITY.md"))
	if err != nil {
		t.Fatal(err)
	}
	repoTestsReadme, err := os.ReadFile(filepath.Join(root, "repotests", "README.md"))
	if err != nil {
		t.Fatal(err)
	}
	geminiStarterReadme, err := os.ReadFile(filepath.Join(root, "cli", "internal", "scaffold", "templates", "gemini.README.go.md.tmpl"))
	if err != nil {
		t.Fatal(err)
	}

	mustContain(t, string(productionDoc), "Gemini packaging: production-ready official Gemini CLI extension packaging lane")
	mustContain(t, string(productionDoc), "Gemini runtime: optional production-ready 9-hook Go runtime lane for `SessionStart`, `SessionEnd`, `BeforeModel`, `AfterModel`, `BeforeToolSelection`, `BeforeAgent`, `AfterAgent`, `BeforeTool`, and `AfterTool`")
	mustContain(t, string(productionDoc), "make test-gemini-runtime")
	mustContain(t, string(productionDoc), "make test-gemini-runtime-live")
	mustContain(t, string(productionDoc), "production-ready 9-hook Gemini Go runtime lane")

	mustContain(t, string(supportDoc), "`github.com/777genius/plugin-kit-ai/sdk/gemini`")
	mustContain(t, string(supportDoc), "`(*plugin-kit-ai.App).Gemini`")
	mustContain(t, string(supportDoc), "Gemini packaging: production-ready official Gemini CLI extension packaging lane")
	mustContain(t, string(supportDoc), "Gemini runtime: optional production-ready 9-hook Go runtime lane for `SessionStart`, `SessionEnd`, `BeforeModel`, `AfterModel`, `BeforeToolSelection`, `BeforeAgent`, `AfterAgent`, `BeforeTool`, and `AfterTool`")
	mustContain(t, string(supportDoc), "[GEMINI_RUNTIME_AUDIT.md](./GEMINI_RUNTIME_AUDIT.md)")
	mustContain(t, string(supportDoc), "- Gemini:\n  - `SessionStart`")

	mustContain(t, string(sdkReadme), "`gemini/SessionStart`")
	mustContain(t, string(sdkReadme), "`gemini/BeforeToolSelection`")
	mustContain(t, string(sdkReadme), "`gemini/BeforeTool`")
	mustContain(t, string(sdkReadme), "`gemini/AfterTool`")
	mustContain(t, string(sdkReadme), "`gemini.BeforeToolSelectionForceAny(...)`")
	mustContain(t, string(sdkReadme), "`gemini.AfterToolTailCallValue(...)`")
	mustContain(t, string(sdkReadme), "promoted 9-hook runtime surface is now also `public-stable`")
	mustContain(t, string(sdkReadme), "[../../docs/GEMINI_RUNTIME_AUDIT.md](../../docs/GEMINI_RUNTIME_AUDIT.md)")
	mustContain(t, string(sdkReadme), "Gemini's current production-ready 9-hook runtime boundary is audited")

	mustContain(t, string(sdkStability), "`(*plugin-kit-ai.App).Gemini`")
	mustContain(t, string(sdkStability), "approved exported Gemini event and response types for:")
	mustContain(t, string(sdkStability), "approved exported Gemini helper constructors for the stable 9-hook runtime surface")
	mustContain(t, string(sdkStability), "`BeforeToolSelectionForceAny`")
	mustContain(t, string(sdkStability), "`AfterToolTailCallValue`")
	mustContain(t, string(sdkStability), "approved exported Gemini event/response/helper surfaces for the promoted 9-hook runtime")

	mustContain(t, string(repoTestsReadme), "`PLUGIN_KIT_AI_RUN_GEMINI_RUNTIME_LIVE=1`")
	mustContain(t, string(repoTestsReadme), "`PLUGIN_KIT_AI_E2E_GEMINI`")
	mustContain(t, string(repoTestsReadme), "make test-gemini-runtime")
	mustContain(t, string(repoTestsReadme), "make test-gemini-runtime-live")
	mustContain(t, string(repoTestsReadme), "current production-ready 9-hook runtime")
	mustContain(t, string(repoTestsReadme), "blocked-tool control semantics")
	mustContain(t, string(repoTestsReadme), "blocked-model control semantics")
	mustContain(t, string(repoTestsReadme), "`mode:\"NONE\"` semantics")
	mustContain(t, string(repoTestsReadme), "current production-ready 9-hook runtime")

	mustContain(t, string(geminiStarterReadme), "Packaging claim: `production-ready extension packaging lane`")
	mustContain(t, string(geminiStarterReadme), "Runtime claim: `production-ready 9-hook Go runtime lane`")
	mustContain(t, string(geminiStarterReadme), "production-ready 9-hook Go runtime")
	mustContain(t, string(geminiStarterReadme), "make test-gemini-runtime")
	mustContain(t, string(geminiStarterReadme), "make test-gemini-runtime-live")
	mustContain(t, string(geminiStarterReadme), "`plugin-kit-ai capabilities --mode runtime --platform gemini`")
	mustContain(t, string(geminiStarterReadme), "production-ready 9-hook Go runtime")
}
