package report

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// A manager diagnostic must fail validation without assigning manager rules to
// normative conformance or exposing dependency bytes in the public report.
func TestDependencyDiagnosticsFailReadinessOnly(t *testing.T) {
	cleanFiles := map[string]string{"plugin.json": commandsCore, "skills/guide/SKILL.md": guideSkill}
	clean := Build("validate", "test", readCommandProject(t, cleanFiles), false)
	for _, tc := range []struct{ name, plugin, toml, code, location string }{
		{"invalid TOML", commandsCore, "# author note\n\nschema_version = 999\n", "dependency_manifest_invalid", "uap.toml:3"},
		{"retired JSON only", strings.TrimSuffix(commandsCore, "}") + `,"extensions":{"` + domain.ExtensionNamespace + `":{"dependencies":null}}}`, "", "dependency_declaration_moved", "plugin.json"},
		{"retired JSON and TOML", strings.TrimSuffix(commandsCore, "}") + `,"extensions":{"` + domain.ExtensionNamespace + `":{"dependencies":{"acme/context":"^2.0"}}}}`, "schema_version = 1\n", "dependency_declaration_moved", "plugin.json"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			files := map[string]string{"plugin.json": tc.plugin, "skills/guide/SKILL.md": guideSkill}
			if tc.toml != "" {
				files["uap.toml"] = tc.toml
			}
			p := readCommandProject(t, files)
			r := Build("validate", "test", p, true)
			if r.Readiness.Status != Fail || r.Release.Status != Fail || r.Conformance.Status != clean.Conformance.Status || r.HostSafety.Status != clean.HostSafety.Status || r.Loadability.Status != Pass {
				t.Fatalf("manager diagnostic crossed boundary: %+v", r)
			}
			count := 0
			for _, finding := range r.Findings {
				if finding.Layer == "package_management" {
					count++
					if finding.Code != tc.code || finding.Location != tc.location {
						t.Fatalf("finding = %+v", finding)
					}
				}
			}
			if count != 1 {
				t.Fatalf("manager finding count = %d", count)
			}
			public := r.PublicResult("author.validate", "read", false, nil)
			body, err := json.Marshal(public)
			if err != nil || strings.Contains(string(body), "acme/context") || !strings.Contains(string(body), tc.code) || public.Readiness.Status != Fail {
				t.Fatalf("public diagnostics lost or exposed data: %s %v", body, err)
			}
		})
	}
}
