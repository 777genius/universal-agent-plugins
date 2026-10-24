package sourceacquisition

import (
	"fmt"
	"path"
	"strings"

	"golang.org/x/text/cases"
	"golang.org/x/text/unicode/norm"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/pathpolicy"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// validateWebSkillTreePaths applies the same Unicode portability boundary as
// the snapshot builder before file downloads or filesystem materialization.
func validateWebSkillTreePaths(files []domain.WebSkillFile) error {
	type entry struct {
		spelling string
		file     bool
	}
	seen := make(map[string]entry)
	for _, file := range files {
		if err := validateWebSkillPortableSegments(file.Path); err != nil {
			return err
		}
		for prefix := file.Path; prefix != "."; prefix = path.Dir(prefix) {
			key := cases.Fold().String(norm.NFC.String(prefix))
			current := entry{spelling: prefix, file: prefix == file.Path}
			if previous, exists := seen[key]; exists && (previous.spelling != prefix || previous.file || current.file) {
				return fmt.Errorf("skill tree contains a portable file or directory collision")
			}
			seen[key] = current
		}
	}
	return nil
}

func validateWebSkillPortableSegments(value string) error {
	for _, segment := range strings.Split(value, "/") {
		if err := pathpolicy.ValidatePortablePathSegment(segment); err != nil {
			return fmt.Errorf("skill file path is not portable: %w", err)
		}
	}
	return nil
}
