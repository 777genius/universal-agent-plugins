package dependencymanifest

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
)

// ErrDependencyDeclarationMoved identifies retired JSON dependency declarations.
var ErrDependencyDeclarationMoved = errors.New("dependency_declaration_moved")

const legacyNamespace = "io.github.777genius.agentplugins"

// RejectLegacyDependencies rejects presence of the retired field regardless of
// its value. Raw extension data remains untouched, including malformed values.
func RejectLegacyDependencies(extensions map[string]json.RawMessage) error {
	raw, exists := extensions[legacyNamespace]
	if !exists {
		return nil
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	token, err := decoder.Token()
	if err != nil {
		return fmt.Errorf("%s extension: %w", legacyNamespace, err)
	}
	if token != json.Delim('{') {
		return nil
	}
	for decoder.More() {
		token, err = decoder.Token()
		if err != nil {
			return fmt.Errorf("%s extension: %w", legacyNamespace, err)
		}
		if token == "dependencies" {
			return fmt.Errorf("%w: move %s.dependencies to uap.toml [dependencies]", ErrDependencyDeclarationMoved, legacyNamespace)
		}
		var ignored json.RawMessage
		if err := decoder.Decode(&ignored); err != nil {
			return fmt.Errorf("%s extension: %w", legacyNamespace, err)
		}
	}
	return nil
}
