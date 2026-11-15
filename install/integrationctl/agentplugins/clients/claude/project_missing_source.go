package claude

import (
	"fmt"
	"os"
)

// Claude 2.1.263 and 2.1.325 retain the exact cache registration when their directory
// marketplace disappears. This is unavailable content, never active evidence.
// Registry admission additionally requires the prior receipt, native index,
// unchanged cache digest and physical source absence.
func claudeMarketplaceSourceMissing(entry map[string]any, marketplace string) bool {
	projectEnabled, hasProjectEnabled := entry["projectEnabled"]
	detailValue, hasDetails := entry["errorDetails"]
	// The older captured schema omits both members; partial newer metadata is unknown.
	if hasProjectEnabled != hasDetails || hasProjectEnabled && projectEnabled != true {
		return false
	}
	if _, exists := entry["readFromFolder"]; exists {
		return false
	}
	if _, exists := entry["folderVersion"]; exists {
		return false
	}
	errors, ok := entry["errors"].([]any)
	if !ok || len(errors) != 1 || errors[0] != "Marketplace "+marketplace+" failed to load: cache-miss" {
		return false
	}
	if hasDetails {
		details, ok := detailValue.([]any)
		if !ok || len(details) != 1 {
			return false
		}
		detail, ok := details[0].(map[string]any)
		if !ok || len(detail) != 2 || detail["type"] != "marketplace-load-failed" || detail["marketplace"] != marketplace {
			return false
		}
	}
	notes, present := entry["notes"]
	if !present {
		return true
	}
	values, ok := notes.([]any)
	return ok && len(values) == 0
}

func requireMissingClaudeProjectSource(path string) error {
	_, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	return fmt.Errorf("the Claude project source is present despite its native cache-miss diagnostic")
}
