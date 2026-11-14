package statemigration

import (
	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/pathpolicy"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"testing"
	"time"
)

func TestLegacyPhysicalIdentitySeedAndInvalidNames(t *testing.T) {
	for _, name := range []string{"ordinary", "con.foo", "CON.foo", "con.bad_name", "con..bad"} {
		record := legacyInstallation{IntegrationID: name}
		// Exercise the real migration decoder to avoid inventing target fields.
		record.Targets = map[string]legacyTarget{"cursor": {TargetID: "cursor"}}
		installation, err := migrateInstallation(record, time.Now(), func() (string, error) { return "installation", nil })
		if err != nil {
			t.Fatal(err)
		}
		for _, binding := range installation.Clients {
			want := domain.ComputePhysicalArtifactID(name, "installation\x00cursor")
			if binding.PhysicalArtifact != want {
				t.Fatalf("migration seed changed: %q != %q", binding.PhysicalArtifact, want)
			}
			if name != "ordinary" && name != "con.foo" && pathpolicy.ValidateLeafID(binding.PhysicalArtifact) == nil {
				t.Fatalf("invalid legacy name sanitized: %s", name)
			}
		}
	}
}
