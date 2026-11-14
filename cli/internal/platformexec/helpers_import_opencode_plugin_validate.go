package platformexec

import (
	"fmt"
	"strings"
)

func validateOpenCodePluginRefs(refs []opencodePluginRef) error {
	for i, ref := range refs {
		if strings.TrimSpace(ref.Name) == "" {
			return fmt.Errorf("plugin entry %d must define a non-empty name", i)
		}
		if ref.Options == nil {
			continue
		}
		for key := range ref.Options {
			if strings.TrimSpace(key) == "" {
				return fmt.Errorf("plugin entry %d options may not contain empty keys", i)
			}
		}
	}
	return nil
}
