package usecase

import "fmt"

func actionNamePrefix(action string) string {
	switch action {
	case "update_version":
		return "update"
	case "remove_orphaned_target":
		return "remove"
	case "repair_drift":
		return "repair"
	case "enable_target":
		return "enable"
	case "disable_target":
		return "disable"
	default:
		return "operation"
	}
}

func summaryForExisting(action, integrationID string) string {
	switch action {
	case "update_version":
		return fmt.Sprintf("Updated integration %q.", integrationID)
	case "remove_orphaned_target":
		return fmt.Sprintf("Removed managed targets from integration %q.", integrationID)
	case "repair_drift":
		return fmt.Sprintf("Repaired managed targets for integration %q.", integrationID)
	case "enable_target":
		return fmt.Sprintf("Enabled managed targets for integration %q.", integrationID)
	case "disable_target":
		return fmt.Sprintf("Disabled managed targets for integration %q.", integrationID)
	default:
		return fmt.Sprintf("Applied %s for %q.", action, integrationID)
	}
}
