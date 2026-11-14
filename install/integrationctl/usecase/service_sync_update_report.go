package usecase

import "fmt"

func batchUpdateSkipWarning(integrationID string, err error) string {
	return fmt.Sprintf("Update skipped for %s: %v", integrationID, err)
}
