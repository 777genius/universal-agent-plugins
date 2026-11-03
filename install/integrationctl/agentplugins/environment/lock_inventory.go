package environment

import (
	"fmt"
	"path"
	"strings"

	"golang.org/x/text/cases"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/sourcespec"
)

// ValidateExecutableInventory checks portable package-member names without
// accessing their bytes. Absence means an empty inventory. Input order is not
// authoritative, but duplicate or case-colliding members are never admissible.
// Actual file membership and executable modes require a verified package tree.
func ValidateExecutableInventory(files []string) error {
	seen := make(map[string]bool, len(files))
	fold := cases.Fold()
	for _, name := range files {
		if name == "" || name == "." || path.IsAbs(name) || path.Clean(name) != name {
			return fmt.Errorf("invalid executable inventory path %q", name)
		}
		parts := strings.Split(name, "/")
		if len(parts) > domain.DefaultMaxDepth {
			return fmt.Errorf("executable inventory path exceeds package depth: %q", name)
		}
		for _, part := range parts {
			// Reuse source-coordinate segment portability without its 1,024-byte
			// whole-subpath ceiling: package member names can be longer.
			if _, err := sourcespec.NormalizeSubpath(part); err != nil || strings.EqualFold(part, ".git") {
				return fmt.Errorf("invalid executable inventory path %q", name)
			}
		}
		key := fold.String(name)
		if seen[key] {
			return fmt.Errorf("duplicate executable inventory path %q", name)
		}
		seen[key] = true
	}
	return nil
}
