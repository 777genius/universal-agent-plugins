package statev2

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func TestLegacyMutationReadinessReportsMigrationSchemas(t *testing.T) {
	for _, schema := range []int{2, 3, 4} {
		t.Run(fmt.Sprintf("schema-%d", schema), func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "state-v2.json")
			if err := os.WriteFile(path, []byte(fmt.Sprintf(`{"schema_version":%d,"installations":[]}`, schema)), 0o600); err != nil {
				t.Fatal(err)
			}
			store := Store{Path: path}
			if _, err := store.Load(); err != nil {
				t.Fatalf("old schema is not readable: %v", err)
			}
			err := store.RequireMutationReady()
			var required *MigrationRequiredError
			if !errors.Is(err, ErrMigrationRequired) || !errors.As(err, &required) {
				t.Fatalf("mutation refusal has no typed schema information: %v", err)
			}
			if required.SourceSchema != schema || required.TargetSchema != domain.StateSchemaVersion {
				t.Fatalf("migration schemas = %d -> %d", required.SourceSchema, required.TargetSchema)
			}
		})
	}
}
