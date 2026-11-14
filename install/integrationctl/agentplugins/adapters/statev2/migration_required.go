package statev2

import (
	"errors"
	"fmt"
)

// ErrMigrationRequired classifies a readable state that an explicit migration
// must upgrade before any mutation.
var ErrMigrationRequired = errors.New("migration_required")

// MigrationRequiredError identifies the on-disk and required writer schemas.
// Read-only loading remains available for supported older schemas.
type MigrationRequiredError struct {
	SourceSchema int
	TargetSchema int
}

func (err *MigrationRequiredError) Error() string {
	return fmt.Sprintf("%s: state schema %d requires explicit migration to schema %d; run agentplugins migrate-state --dry-run, then agentplugins migrate-state", ErrMigrationRequired, err.SourceSchema, err.TargetSchema)
}

func (err *MigrationRequiredError) Unwrap() error { return ErrMigrationRequired }
