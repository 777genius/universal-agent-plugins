package main

import (
	"fmt"
	"path/filepath"

	"github.com/777genius/plugin-kit-ai/cli/internal/publicationmodel"
)

func diagnoseMissingPublicationArtifacts(root string, model publicationmodel.Model) []publicationIssue {
	issues := diagnoseMissingPublicationPackageArtifacts(root, model.Packages)
	return append(issues, diagnoseMissingPublicationChannelArtifacts(root, model.Channels)...)
}

func diagnoseMissingPublicationPackageArtifacts(root string, packages []publicationmodel.Package) []publicationIssue {
	var issues []publicationIssue
	for _, pkg := range packages {
		if issue, ok := diagnoseMissingPublicationPackageArtifact(root, pkg); ok {
			issues = append(issues, issue)
		}
	}
	return issues
}

func diagnoseMissingPublicationChannelArtifacts(root string, channels []publicationmodel.Channel) []publicationIssue {
	var issues []publicationIssue
	for _, channel := range channels {
		if issue, ok := diagnoseMissingPublicationChannelArtifact(root, channel); ok {
			issues = append(issues, issue)
		}
	}
	return issues
}

func diagnoseMissingPublicationPackageArtifact(root string, pkg publicationmodel.Package) (publicationIssue, bool) {
	path := expectedPackageArtifactPath(pkg.Target)
	if path == "" || fileExists(filepath.Join(root, path)) {
		return publicationIssue{}, false
	}
	return publicationIssue{
		Code:    "missing_package_artifact",
		Target:  pkg.Target,
		Path:    path,
		Message: fmt.Sprintf("target %s is missing generated package artifact %s", pkg.Target, path),
	}, true
}

func diagnoseMissingPublicationChannelArtifact(root string, channel publicationmodel.Channel) (publicationIssue, bool) {
	path := expectedChannelArtifactPath(channel.Family)
	if path == "" || fileExists(filepath.Join(root, path)) {
		return publicationIssue{}, false
	}
	return publicationIssue{
		Code:          "missing_channel_artifact",
		ChannelFamily: channel.Family,
		Path:          path,
		Message:       fmt.Sprintf("channel %s is missing generated publication artifact %s", channel.Family, path),
	}, true
}
