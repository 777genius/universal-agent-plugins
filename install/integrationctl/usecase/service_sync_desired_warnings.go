package usecase

func syncDesiredAddFailedWarning(integrationID string, err error) string {
	return "Sync add failed for " + integrationID + ": " + err.Error()
}

func syncDesiredReplaceBlockedWarning(integrationID string) string {
	return "Sync skipped replace for " + integrationID + ": replacing non-project scoped integrations is blocked"
}

func syncDesiredRemoveBeforeAddWarning(integrationID string, err error) string {
	return "Sync remove-before-add failed for " + integrationID + ": " + err.Error()
}

func syncDesiredReAddFailedWarning(integrationID string, err error) string {
	return "Sync re-add failed for " + integrationID + ": " + err.Error()
}

func syncDesiredUpdateFailedWarning(integrationID string, err error) string {
	return "Sync update failed for " + integrationID + ": " + err.Error()
}
