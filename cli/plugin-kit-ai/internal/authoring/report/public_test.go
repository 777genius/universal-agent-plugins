package report

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/cli/internal/authoring/project"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/conformance"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func TestPublicProjectionReferencesAndCleanup(t *testing.T) {
	r := New("validate", "revision")
	id := r.add(Finding{Code: "plugin_manifest_missing", Layer: "normative", Rule: "core", Location: "plugin.json", Severity: "error"})
	r.Conformance = Assessment{Fail, []string{id}}
	r.Loadability = r.Conformance
	r.Readiness = r.Conformance
	r.Release = r.Conformance
	r.Checks = []Check{{"portable_configuration", r.Conformance}}
	before, _ := json.Marshal(r)
	p := r.PublicResult("author.validate", "read", true, nil)
	if len(p.Findings) != 1 || p.Findings[0].Code != "missing_standard_manifest" || p.Findings[0].ID == id {
		t.Fatalf("projection: %+v", p)
	}
	for _, a := range []Assessment{p.Conformance, p.Loadability, p.Readiness, p.Release, p.Checks[0].Assessment} {
		if !reflect.DeepEqual(a.FindingIDs, []string{p.Findings[0].ID}) || a.Status != Fail {
			t.Fatalf("reference or policy: %+v", a)
		}
	}
	after, _ := json.Marshal(r)
	if string(before) != string(after) {
		t.Fatal("public projection mutated private report")
	}
	r = New("init", "revision")
	r.Committed = true
	r.Paths = []string{"plugin.json", "skills/demo/SKILL.md"}
	r.AddError("private_cleanup_failed", "Inspect the committed package and owned staging before retrying.")
	p = r.PublicResult("author.init", "local_mutation", true, nil)
	if !p.Committed || !p.Effects.Committed || !p.Effects.Attempted || !reflect.DeepEqual(p.Paths, r.Paths) || p.Error.Code != "private_cleanup_failed" || p.Conformance.Status != NotEvaluated {
		t.Fatalf("postcommit evidence lost: %+v", p)
	}
}

func TestPublicDisplayBoundsAndWithholding(t *testing.T) {
	for _, s := range []string{"demo", "docs-helper", "com.example.docs", "preview-1", "1.2+build", "node", "UPPER"} {
		if DisplayIdentity(s) != s {
			t.Errorf("ordinary identity withheld: %s", s)
		}
	}
	for _, s := range []string{"ghp_aabbcc", "github_pat_aabbcc", "sk-private", "my-secret", "AKIA123456789", "Bearer-abcd", "hello\nworld", "../private", "/tmp/private", "C:\\private", "https://private.example", strings.Repeat("x", 65), strings.Repeat("z", 25)} {
		if DisplayIdentity(s) != "" {
			t.Errorf("unsafe display: %q", s)
		}
	}
	pkg := &domain.PackageEnvelope{SchemaURI: domain.PluginSchemaV1, Manifest: domain.PluginManifest{Name: "demo", Version: "preview-1"}}
	pkg.MCP.Servers = map[string]domain.MCPServer{
		"local":   {Type: "stdio", StdioRequirement: &domain.StdioRequirement{Command: "node", Kind: domain.ExecutableBare}, Decoded: map[string]any{"args": []string{"SECRET"}, "env": map[string]any{"PASSWORD": "SECRET"}}},
		"bundled": {Type: "stdio", StdioRequirement: &domain.StdioRequirement{Command: "/SECRET/private", Kind: domain.ExecutableBundled, BundledRelativePath: "SECRET/private"}},
	}
	pkg.Manifest.Extensions = map[string]json.RawMessage{"com.example.docs": json.RawMessage(`{"SECRET":"SECRET"}`)}
	p := project.Result{Facts: conformance.Facts{Package: pkg}}
	d := inspection(p)
	body, _ := json.Marshal(d)
	if strings.Contains(string(body), "SECRET") || !strings.Contains(string(body), `"executable":"node"`) || !strings.Contains(string(body), `"namespace":"com.example.docs"`) {
		t.Fatalf("display: %s", body)
	}
	pkg.Inventory.InvalidSkills = make([]string, 200)
	for i := range pkg.Inventory.InvalidSkills {
		pkg.Inventory.InvalidSkills[i] = strings.Repeat("x", i+1)
	}
	first := inspection(p)
	second := inspection(p)
	if !first.Truncated || len(first.Components) != 128 || !reflect.DeepEqual(first, second) {
		t.Fatal("unbounded or unstable display")
	}
}

func TestPublicOperationErrorsNeverEvaluatePolicies(t *testing.T) {
	for _, code := range []string{"arguments_invalid", "canceled", "plugin_identity_invalid"} {
		r := New("init", "revision")
		r.AddOperationError(code, "Choose explicit safe inputs.")
		p := r.PublicResult("author.init", "local_mutation", false, nil)
		if p.HostSafety.Status != NotEvaluated || p.Readiness.Status != NotEvaluated || p.Conformance.Status != NotEvaluated || p.Findings[0].Layer != "operation" || p.Effects.Attempted {
			t.Fatalf("operation changed policies: %+v", p)
		}
	}
}

func TestPublicAffectedPathsWithholdCredentialNames(t *testing.T) {
	r := New("skills init", "revision")
	r.Committed = true
	r.Paths = []string{"skills/token-private/SKILL.md", "plugin.json"}
	p := r.PublicResult("author.skills.init", "local_mutation", true, nil)
	b, _ := json.Marshal(p)
	if strings.Contains(string(b), "token-private") || len(p.WithheldPathIDs) != 1 || !reflect.DeepEqual(p.Paths, []string{"plugin.json"}) || !p.Committed {
		t.Fatalf("affected paths: %s", b)
	}
}

// Freeze complete payload bytes in addition to the independently asserted
// semantics above. Full documents are logged for review in immutable test logs.
func TestPublicGoldenResults(t *testing.T) {
	for _, tc := range []struct{ name, hash string }{
		{"success", "402d8caeae2e5e56fc276391f635ab1c5da157a743f3ed73c5d11ce1d90ac42c"}, {"policy_failure", "6f7549eae5eab9fe44c75e95cc2b7cf284694f996a57140be13abd7d8c7032a2"}, {"syntax", "1bd9fb42dc70764d46f0f7a7181634a3bea43546d52b4ba48337d398c48e8740"}, {"missing_standard", "1c5385081a5180efc89c67c3d0cac3df44fb043dd34c9d237d22ecc81900d0b9"}, {"help", "ad5d089f528e1613cf34bc82edc0bb99fc7700bc476951087550cfc737ed91d8"}, {"postcommit_cleanup", "4d9572e97a8ef633f48c58143c9b7e6d61fb0e6e03e8bbc5524505b81ecce527"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := New("inspect", "exact-revision")
			operation, mode, attempted := "author.inspect", "read", true
			switch tc.name {
			case "success":
				r.Loadability = assessment(Pass)
				r.Conformance = assessment(Pass)
				r.HostSafety = assessment(Pass)
				r.Readiness = assessment(Pass)
			case "policy_failure":
				id := r.add(Finding{Code: "mcp_server_invalid", Layer: "normative", Rule: "mcp/server", Severity: "error"})
				r.Conformance = Assessment{Fail, []string{id}}
				r.Readiness = r.Conformance
				r.Loadability = assessment(Pass)
			case "syntax":
				operation, mode, attempted = "author.init", "local_mutation", false
				r.AddOperationError("arguments_invalid", "Choose an implemented template.")
			case "missing_standard":
				id := r.add(Finding{Code: "plugin_manifest_missing", Layer: "normative", Rule: "core", Location: "plugin.json", Severity: "error"})
				r.Conformance = Assessment{Fail, []string{id}}
				r.Loadability = r.Conformance
				r.Legacy = "present"
			case "help":
				operation, attempted = "author.skills", false
			case "postcommit_cleanup":
				operation, mode = "author.init", "local_mutation"
				r.Committed = true
				r.Paths = []string{"plugin.json"}
				r.AddError("private_cleanup_failed", "Inspect owned staging before retrying.")
			}
			body, err := json.Marshal(r.PublicResult(operation, mode, attempted, nil))
			if err != nil {
				t.Fatal(err)
			}
			sum := fmt.Sprintf("%x", sha256.Sum256(body))
			t.Logf("PUBLIC_GOLDEN %s %s %s", tc.name, sum, body)
			if sum != tc.hash {
				t.Fatalf("public schema changed: %s", sum)
			}
		})
	}
}
