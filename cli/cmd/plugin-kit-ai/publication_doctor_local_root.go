package main

import "github.com/777genius/plugin-kit-ai/cli/internal/app"

func maybeVerifyPublicationLocalRoot(runner inspectRunner, root, requestedTarget, dest, packageRoot, diagnosisStatus string) (*app.PluginPublicationVerifyRootResult, error) {
	result, ok, err := verifyPublicationLocalRootResult(runner, root, requestedTarget, dest, packageRoot, diagnosisStatus)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, nil
	}
	return &result, nil
}

func mergePublicationDiagnosisWithLocalRoot(diagnosis publicationDiagnosis, requestedTarget string, localRoot *app.PluginPublicationVerifyRootResult) publicationDiagnosis {
	if !shouldMergePublicationLocalRoot(localRoot) {
		return diagnosis
	}
	return publicationDiagnosisWithLocalRoot(diagnosis, requestedTarget, localRoot)
}

func normalizePublicationLocalRoot(localRoot *app.PluginPublicationVerifyRootResult) *app.PluginPublicationVerifyRootResult {
	return normalizedPublicationLocalRoot(localRoot)
}

func verifyPublicationLocalRootResult(runner inspectRunner, root, requestedTarget, dest, packageRoot, diagnosisStatus string) (app.PluginPublicationVerifyRootResult, bool, error) {
	if !shouldVerifyPublicationLocalRoot(dest, diagnosisStatus) {
		return app.PluginPublicationVerifyRootResult{}, false, nil
	}
	result, err := verifyPublicationLocalRootWithRunner(runner, publicationLocalRootOptions(root, requestedTarget, dest, packageRoot))
	if err != nil {
		return app.PluginPublicationVerifyRootResult{}, false, err
	}
	return result, true, nil
}

func shouldMergePublicationLocalRoot(localRoot *app.PluginPublicationVerifyRootResult) bool {
	return localRoot != nil
}

func publicationDiagnosisWithLocalRoot(diagnosis publicationDiagnosis, requestedTarget string, localRoot *app.PluginPublicationVerifyRootResult) publicationDiagnosis {
	diagnosis = mergePublicationDiagnosisLocalRootStatus(diagnosis, localRoot)
	diagnosis.Issues = mergePublicationDiagnosisLocalRootIssues(diagnosis.Issues, requestedTarget, localRoot)
	diagnosis.NextSteps = mergePublicationDiagnosisLocalRootNextSteps(diagnosis.NextSteps, localRoot)
	return diagnosis
}

func mergePublicationDiagnosisLocalRootIssues(issues []publicationIssue, requestedTarget string, localRoot *app.PluginPublicationVerifyRootResult) []publicationIssue {
	return append(issues, localRootPublicationIssues(requestedTarget, localRoot)...)
}
