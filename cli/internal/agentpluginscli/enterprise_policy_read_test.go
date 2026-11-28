package agentpluginscli

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/enterprisepolicy"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/sourceacquisition"
)

func TestEnterprisePolicyDoesNotBlockPackageValidation(t *testing.T) {
	for _, body := range []string{
		"schema_version: 2\nsources:\n  allowed_kinds: [git]\n",
		"schema_version: 2\nunknown_policy_rule: true\n",
	} {
		t.Run(body, func(t *testing.T) {
			fixture := newCLIFixture(t, nil)
			fixture.app.Detector = dependencyCheckForbiddenEffects{}
			// Validation acquires an ephemeral snapshot. Keep its temporary directory
			// separate so the observation covers managed state, not scratch churn.
			fixture.app.SourceAcquirer = sourceacquisition.Acquirer{TempRoot: t.TempDir()}
			path := filepath.Join(fixture.root, "policy.yaml")
			dependencyWrite(t, path, body)
			setEnterpriseCommandFiles(&fixture, enterprisepolicy.Path{Layer: "machine", Path: path})
			plugin := writeCLIPlugin(t)
			before := observeDependencyCheckFiles(t, fixture.root)
			output, stderr, err := fixture.execute(false, "validate", plugin, "--format", "json")
			if err != nil || !strings.Contains(output, `"conformant":true`) && !strings.Contains(output, `"conformant": true`) {
				t.Fatalf("read-only validation was blocked: %v\n%s\n%s", err, output, stderr)
			}
			assertDependencyCheckFilesUnchanged(t, fixture.root, before)
			if _, _, err := fixture.execute(false, "policy", "check", plugin, "--target", "codex", "--format", "json"); err == nil {
				t.Fatal("read-only validation removed policy from the original application")
			}
		})
	}
}
