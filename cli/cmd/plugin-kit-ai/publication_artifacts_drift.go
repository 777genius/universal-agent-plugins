package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"

	"github.com/777genius/plugin-kit-ai/cli/internal/pluginmanifest"
	"github.com/777genius/plugin-kit-ai/cli/internal/publicationmodel"
)

func diagnoseGeneratedPublicationArtifacts(root, requestedTarget string, model publicationmodel.Model) []publicationIssue {
	generated, err := pluginmanifest.Generate(root, normalizePublicationRequestedTarget(requestedTarget))
	if err != nil {
		return []publicationIssue{{
			Code:    "generate_probe_failed",
			Path:    pluginmanifest.FileName,
			Message: fmt.Sprintf("publication doctor could not probe generated publication artifacts: %v", err),
		}}
	}
	issues := diagnosePublicationArtifactBodies(root, model, expectedPublicationArtifactBodies(generated.Artifacts))
	return append(issues, diagnosePublicationStaleArtifacts(generated.StalePaths)...)
}

func expectedPublicationArtifactBodies(artifacts []pluginmanifest.Artifact) map[string][]byte {
	expectedBodies := make(map[string][]byte, len(artifacts))
	for _, artifact := range artifacts {
		expectedBodies[artifact.RelPath] = artifact.Content
	}
	return expectedBodies
}

func diagnosePublicationStaleArtifacts(paths []string) []publicationIssue {
	var issues []publicationIssue
	for _, path := range paths {
		if !isPublicationRelevantPath(path) {
			continue
		}
		issues = append(issues, publicationIssue{
			Code:    "stale_generated_artifact",
			Path:    path,
			Message: fmt.Sprintf("generated publication artifact %s is stale and should be removed by generate", path),
		})
	}
	return issues
}

func diagnosePublicationArtifactBodies(root string, model publicationmodel.Model, expectedBodies map[string][]byte) []publicationIssue {
	var issues []publicationIssue
	for _, pkg := range model.Packages {
		if path := expectedPackageArtifactPath(pkg.Target); path != "" {
			if issue, ok := diagnosePublicationArtifactDrift(root, path, expectedBodies[path], "drifted_package_artifact"); ok {
				issue.Target = pkg.Target
				issues = append(issues, issue)
			}
		}
	}
	for _, channel := range model.Channels {
		if path := expectedChannelArtifactPath(channel.Family); path != "" {
			if issue, ok := diagnosePublicationArtifactDrift(root, path, expectedBodies[path], "drifted_channel_artifact"); ok {
				issue.ChannelFamily = channel.Family
				issues = append(issues, issue)
			}
		}
	}
	return issues
}

func diagnosePublicationArtifactDrift(root, path string, expected []byte, code string) (publicationIssue, bool) {
	if len(expected) == 0 || !fileExists(filepath.Join(root, path)) {
		return publicationIssue{}, false
	}
	current, err := os.ReadFile(filepath.Join(root, path))
	if err != nil || bytes.Equal(current, expected) {
		return publicationIssue{}, false
	}
	return publicationIssue{
		Code:    code,
		Path:    path,
		Message: fmt.Sprintf("generated publication artifact %s is out of sync with current authored inputs", path),
	}, true
}
