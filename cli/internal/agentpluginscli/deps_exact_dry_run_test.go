package agentpluginscli

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// Exact release resolution must supply the graph's actual targets while reading
// its newly acquired manifest, including the targetless seed from saved add.
func TestDependencyExactRegistryDryRunPreservesTargets(t *testing.T) {
	for _, command := range []string{"lock", "saved add"} {
		for _, target := range []string{"codex", "cursor"} {
			t.Run(command+"/"+target, func(t *testing.T) {
				fixture, repo, _ := dependencyReviewFixture(t)
				freshRestorePackage(t, fixture, repo, "a", "1.0.0", "a-1.0.0", "", 1, true)
				args := []string{"add", "acme/a", "--target", target, "--dry-run", "--format", "json"}
				if command == "lock" {
					manifest, _ := fixture.app.dependencyPaths()
					dependencyWrite(t, manifest, "schema_version = 1\n[dependencies.a]\nregistry = \"uap\"\ndistribution = \"acme/a\"\nrelease = \"1\"\n[targets]\n"+target+" = [\"a\"]\n")
					args = []string{"deps", "lock", "--dry-run", "--format", "json"}
				}
				before := freshRestoreFiles(t, fixture.root, fixture.client.ConfigRoot)
				out, stderr, err := fixture.execute(false, args...)
				if target == "cursor" {
					if !errors.Is(err, domain.ErrDirectoryIneligible) || !strings.Contains(err.Error(), target) {
						t.Fatalf("dry-run accepted a target absent from signed policy: %v", err)
					}
				} else if err != nil {
					t.Fatalf("exact registry %s dry-run: %v\n%s\n%s", command, err, out, stderr)
				}
				if err == nil {
					var result struct {
						DryRun bool `json:"dry_run"`
						Data   struct {
							DryRun bool `json:"dry_run"`
						} `json:"data"`
					}
					if decodeErr := json.Unmarshal([]byte(out), &result); decodeErr != nil || !result.DryRun && !result.Data.DryRun {
						t.Fatalf("dry-run not rendered: %s (%v)", out, decodeErr)
					}
				}
				freshRestoreAssertNoEffects(t, fixture, before)
			})
		}
	}
}
