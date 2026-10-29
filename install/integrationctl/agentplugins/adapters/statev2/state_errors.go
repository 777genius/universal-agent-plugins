package statev2

import (
	"errors"
	"fmt"
)

// ErrStateCorrupt identifies invalid JSON or state records. I/O and namespace
// failures retain their original classification instead.
var ErrStateCorrupt = errors.New("state_corrupt")

// ErrUnsupportedStateSchema identifies state written in an unsupported schema.
var ErrUnsupportedStateSchema = errors.New("unsupported_state_schema")

// UnsupportedStateSchemaError lets hosts distinguish newer-writer state from
// an unsupported older format without parsing a diagnostic.
type UnsupportedStateSchemaError struct {
	SourceSchema int
	TargetSchema int
}

func (err *UnsupportedStateSchemaError) Error() string {
	if err.SourceSchema > err.TargetSchema {
		return fmt.Sprintf("state schema %d is newer than supported schema %d; use a newer SDK/binary", err.SourceSchema, err.TargetSchema)
	}
	return fmt.Sprintf("unsupported state schema_version %d for writer schema %d; migrate with a compatible previous SDK/binary", err.SourceSchema, err.TargetSchema)
}

func (err *UnsupportedStateSchemaError) Unwrap() error { return ErrUnsupportedStateSchema }
