package catalog

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func TestCatalogLoadsStrictPinnedIndexAndResolvesExactSource(t *testing.T) {
	t.Parallel()
	body := validCatalog()
	sum := sha256.Sum256(body)
	expected := "sha256:" + hex.EncodeToString(sum[:])
	loaded, err := (Loader{CurrentCLIVersion: "0.1.0"}).Load(body, expected)
	if err != nil {
		t.Fatal(err)
	}
	resolved, err := loaded.Resolve("context7")
	if err != nil {
		t.Fatal(err)
	}
	want := "777genius/universal-agent-plugins@0123456789abcdef0123456789abcdef01234567//plugins/context7"
	if resolved.SourceReference != want || resolved.Entry.TreeDigest != digest("a") {
		t.Fatalf("resolution = %+v", resolved)
	}
	if resolved.Hints.OpenAIMCPAuth["context7"].BearerTokenEnvVar != "CONTEXT7_API_KEY" {
		t.Fatalf("hints = %+v", resolved.Hints)
	}
	compatibility := resolved.Hints.Compatibility["cursor"]
	if compatibility.Authentication != "not_required" || compatibility.Package != "native" {
		t.Fatalf("generic compatibility = %+v", resolved.Hints.Compatibility)
	}
	if resolved.Evidence.SchemaVersion != 1 || resolved.Evidence.CatalogVersion != "0.1.0" || resolved.Evidence.Compatibility["cursor"].Authentication != "not_required" {
		t.Fatalf("catalog evidence = %+v", resolved.Evidence)
	}
	resolved.Hints.Compatibility["cursor"] = domain.CatalogCompatibility{}
	if resolved.Evidence.Compatibility["cursor"].Package != "native" {
		t.Fatal("resolution hints alias immutable catalog evidence")
	}
}

func TestCatalogKeepsNewClientsOptionalButRejectsThemInLegacySchemas(t *testing.T) {
	t.Parallel()
	oldCatalog := validCatalog()
	if _, err := (Loader{CurrentCLIVersion: "0.1.0"}).Load(oldCatalog, ""); err != nil {
		t.Fatalf("legacy catalog now requires new clients: %v", err)
	}
	withClaude := strings.Replace(string(oldCatalog),
		`"kiro":{"package":"native","verification":"tested","authentication":"not_required"}`,
		`"kiro":{"package":"native","verification":"tested","authentication":"not_required"},"claude":{"package":"prepared","verification":"tested","authentication":"not_required"}`,
		1,
	)
	if _, err := (Loader{CurrentCLIVersion: "0.1.0"}).Load([]byte(withClaude), ""); err == nil {
		t.Fatal("catalog v1 accepted a client key that released parsers reject")
	}
}

func TestCatalogRejectsChecksumUnknownFieldsTraversalAndSupplyChainConflict(t *testing.T) {
	t.Parallel()
	tests := map[string][]byte{
		"checksum":  validCatalog(),
		"unknown":   []byte(strings.Replace(string(validCatalog()), `"schema_version": 1,`, `"schema_version": 1, "future": true,`, 1)),
		"traversal": []byte(strings.Replace(string(validCatalog()), `"plugins/context7"`, `"../context7"`, 1)),
		"conflict":  conflictCatalog(),
	}
	for name, body := range tests {
		name, body := name, body
		t.Run(name, func(t *testing.T) {
			expected := ""
			if name == "checksum" {
				expected = digest("f")
			}
			if _, err := (Loader{CurrentCLIVersion: "0.1.0"}).Load(body, expected); err == nil {
				t.Fatal("invalid catalog accepted")
			}
		})
	}
}

func TestCatalogEnforcesMinimumCLIVersionAtResolution(t *testing.T) {
	t.Parallel()
	body := []byte(strings.Replace(
		string(validCatalog()),
		`"minimum_cli_version": "0.1.0"`,
		`"minimum_cli_version": "0.2.0"`,
		1,
	))
	loaded, err := (Loader{CurrentCLIVersion: "0.1.0"}).Load(body, "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := loaded.Resolve("context7"); err == nil || !strings.Contains(err.Error(), "requires agentplugins") {
		t.Fatalf("resolution error = %v", err)
	}
}

func TestCatalogLoadsIntegrityBoundChatGPTAppBinding(t *testing.T) {
	t.Parallel()
	loaded, err := (Loader{CurrentCLIVersion: "0.1.6"}).Load(validCatalogWithChatGPT(), "")
	if err != nil {
		t.Fatal(err)
	}
	resolved, err := loaded.Resolve("context7")
	if err != nil {
		t.Fatal(err)
	}
	binding := resolved.Hints.Compatibility["chatgpt"].AppBinding
	if binding == nil || binding.AppKey != "context7" || binding.ID != "asdk_app_context7_123" || binding.MCPURL != "https://example.test/mcp" {
		t.Fatalf("ChatGPT app binding = %+v", binding)
	}
	binding.ID = "connector_mutated"
	if resolved.Evidence.Compatibility["chatgpt"].AppBinding.ID != "asdk_app_context7_123" {
		t.Fatal("mutable hints aliased immutable catalog evidence")
	}
}

func TestCatalogV1RejectsV2ChatGPTFields(t *testing.T) {
	t.Parallel()
	body := strings.Replace(string(validCatalogWithChatGPT()), domain.CatalogSchemaV2, domain.CatalogSchemaV1, 1)
	body = strings.Replace(body, `"schema_version": 2`, `"schema_version": 1`, 1)
	if _, err := (Loader{CurrentCLIVersion: "0.1.0"}).Load([]byte(body), ""); err == nil {
		t.Fatal("catalog v1 accepted v2 ChatGPT fields")
	}
}

func TestCatalogV2RequiresAgentplugins016(t *testing.T) {
	t.Parallel()
	if _, err := (Loader{CurrentCLIVersion: "0.1.5"}).Load(validCatalogWithChatGPT(), ""); err == nil || !strings.Contains(err.Error(), "0.1.6") {
		t.Fatalf("pre-0.1.6 CLI loaded catalog v2: %v", err)
	}
}

func TestCatalogRejectsUnsafeOrMisplacedChatGPTAppBinding(t *testing.T) {
	t.Parallel()
	valid := string(validCatalogWithChatGPT())
	for name, body := range map[string]string{
		"url-query": strings.Replace(valid, `"https://example.test/mcp"`, `"https://example.test/mcp?token=x"`, 1),
		"bad-id":    strings.Replace(valid, `"asdk_app_context7_123"`, `"not/an/app/id"`, 1),
		"unsafe-evidence": strings.Replace(valid,
			`"tests/e2e/results/chatgpt-context7.json"`, `"../outside.json"`, 1),
		"bad-evidence-revision": strings.Replace(valid, strings.Repeat("e", 40), `not-a-commit`, 1),
		"missing-evidence":      strings.Replace(valid, `"tests/e2e/results/chatgpt-context7.json"`, `""`, 1),
		"missing-evidence-revision": strings.Replace(valid,
			strings.Repeat("e", 40), ``, 1),
		"non-chatgpt": strings.Replace(valid, `"authentication":"not_required"}`, `"authentication":"not_required","app_binding":{"app_key":"context7","id":"asdk_app_context7_123","mcp_server":"context7","mcp_url":"https://example.test/mcp","runtime_evidence":"tests/e2e/results/chatgpt-context7.json"}}`, 1),
	} {
		name, body := name, body
		t.Run(name, func(t *testing.T) {
			if _, err := (Loader{CurrentCLIVersion: "0.1.0"}).Load([]byte(body), ""); err == nil {
				t.Fatal("invalid ChatGPT app binding accepted")
			}
		})
	}
}

func TestValidateAppBindingReferenceRejectsInvalidHTTPSAuthority(t *testing.T) {
	t.Parallel()
	valid := domain.CatalogAppBinding{AppKey: "docs", ID: "asdk_app_docs_123", MCPServer: "docs", MCPURL: "https://example.test:443/mcp"}
	if err := ValidateAppBindingReference(valid); err != nil {
		t.Fatalf("valid reference rejected: %v", err)
	}
	for _, mcpURL := range []string{
		"https://:443/mcp",
		"https://example.test:/mcp",
		"https://[::1]:/mcp",
		"https://example.test:0/mcp",
		"https://example.test:65536/mcp",
		"https://example.test:not-a-port/mcp",
	} {
		binding := valid
		binding.MCPURL = mcpURL
		if err := ValidateAppBindingReference(binding); err == nil {
			t.Fatalf("invalid reference URL %q accepted", mcpURL)
		}
	}
}

func TestCatalogRejectsAnyCompatibilityMatrixDeviation(t *testing.T) {
	t.Parallel()
	tests := map[string]string{
		"omission":       `,"kiro":{"package":"native","verification":"tested","authentication":"not_required"}`,
		"extra":          `"kiro":{"package":"native","verification":"tested","authentication":"not_required"}`,
		"mode":           `"vscode":{"package":"prepared"`,
		"authentication": `"vscode":{"package":"prepared","verification":"tested","authentication":"not_required"}`,
	}
	for name, fragment := range tests {
		name, fragment := name, fragment
		t.Run(name, func(t *testing.T) {
			body := string(validCatalog())
			switch name {
			case "omission":
				body = strings.Replace(body, fragment, "", 1)
			case "extra":
				body = strings.Replace(body, fragment, fragment+`,"typo":{"package":"native","verification":"tested","authentication":"not_required"}`, 1)
			case "mode":
				body = strings.Replace(body, fragment, `"vscode":{"package":"native"`, 1)
			case "authentication":
				body = strings.Replace(body, fragment, `"vscode":{"package":"prepared","verification":"tested","authentication":"required"}`, 1)
			}
			if _, err := (Loader{CurrentCLIVersion: "0.1.0"}).Load([]byte(body), ""); err == nil {
				t.Fatal("invalid compatibility matrix accepted")
			}
		})
	}
}

func TestEmbeddedCatalogV2LoadsAllEntries(t *testing.T) {
	t.Parallel()
	body, err := os.ReadFile("../../../../../cli/plugin-kit-ai/cmd/agentplugins/catalog-v2.json")
	if err != nil {
		t.Fatal(err)
	}
	loaded, err := (Loader{CurrentCLIVersion: "0.1.6"}).Load(body, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded.Catalog.Plugins) != 26 {
		t.Fatalf("embedded package count = %d, want 26", len(loaded.Catalog.Plugins))
	}
	if loaded.Catalog.SchemaVersion != SchemaVersionV2 {
		t.Fatalf("embedded catalog schema_version = %d, want %d", loaded.Catalog.SchemaVersion, SchemaVersionV2)
	}
}

func validCatalog() []byte {
	return []byte(`{
  "$schema": "https://github.com/777genius/universal-agent-plugins/schemas/catalog-v1.schema.json",
  "schema_version": 1,
  "catalog_version": "0.1.0",
  "repository": "777genius/universal-agent-plugins",
  "revision": "0123456789abcdef0123456789abcdef01234567",
  "published_at": "2026-08-08T12:00:00Z",
  "plugins": [{
    "name": "context7",
    "version": "1.0.0",
    "agent_plugins_schema": "https://agent-plugins.org/schemas/1.0.0/plugin.schema.json",
    "minimum_cli_version": "0.1.0",
    "source_path": "plugins/context7",
    "tree_digest": "` + digest("a") + `",
    "manifest_digest": "` + digest("b") + `",
    "components": ["mcp"],
    "compatibility": {"codex":{"package":"projected","verification":"tested","authentication":"not_required"},"cursor":{"package":"native","verification":"tested","authentication":"not_required"},"copilot":{"package":"native","verification":"tested","authentication":"not_required"},"vscode":{"package":"prepared","verification":"tested","authentication":"not_required"},"kiro":{"package":"native","verification":"tested","authentication":"not_required"}},
    "openai_mcp_auth": {"context7": {"bearer_token_env_var":"CONTEXT7_API_KEY"}}
  }]
}`)
}

func validCatalogWithChatGPT() []byte {
	body := strings.Replace(string(validCatalog()), domain.CatalogSchemaV1, domain.CatalogSchemaV2, 1)
	body = strings.Replace(body, `"schema_version": 1`, `"schema_version": 2`, 1)
	body = strings.Replace(body, `"minimum_cli_version": "0.1.0"`, `"minimum_cli_version": "0.1.6"`, 1)
	needle := `"kiro":{"package":"native","verification":"tested","authentication":"not_required"}`
	chatgpt := `,"chatgpt":{"package":"projected","verification":"tested","authentication":"not_required","app_binding":{"app_key":"context7","id":"asdk_app_context7_123","mcp_server":"context7","mcp_url":"https://example.test/mcp","runtime_evidence":"tests/e2e/results/chatgpt-context7.json","runtime_evidence_revision":"` + strings.Repeat("e", 40) + `"}}`
	return []byte(strings.Replace(body, needle, needle+chatgpt, 1))
}

func digest(value string) string {
	return "sha256:" + strings.Repeat(value, 64)
}

func conflictCatalog() []byte {
	duplicate := `, {"name":"context7","version":"1.0.0","agent_plugins_schema":"https://agent-plugins.org/schemas/1.0.0/plugin.schema.json","minimum_cli_version":"0.1.0","source_path":"plugins/context7-copy","tree_digest":"` + digest("c") + `","manifest_digest":"` + digest("b") + `","components":["mcp"],"compatibility":{"codex":{"package":"projected","verification":"tested","authentication":"not_required"},"cursor":{"package":"native","verification":"tested","authentication":"not_required"},"copilot":{"package":"native","verification":"tested","authentication":"not_required"},"vscode":{"package":"prepared","verification":"tested","authentication":"not_required"},"kiro":{"package":"native","verification":"tested","authentication":"not_required"}}}`
	return []byte(strings.Replace(string(validCatalog()), "  }]\n}", "  }"+duplicate+"]\n}", 1))
}
