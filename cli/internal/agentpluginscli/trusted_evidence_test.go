package agentpluginscli

import (
	"runtime"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func intendedTrustedDirectoryEvidence(evidence domain.DirectoryEvidence, overrides ...func(*domain.DirectoryEvidence)) domain.DirectoryEvidence {
	for _, override := range overrides {
		override(&evidence)
	}
	if evidence.SchemaVersion == 0 {
		evidence.SchemaVersion = 1
	}
	if evidence.Level != "schema" {
		if evidence.ClientVersion == "" {
			evidence.ClientVersion = "fixture-client"
		}
		if evidence.InstallerVersion == "" {
			evidence.InstallerVersion = "fixture-installer"
		}
		if evidence.OS == "" {
			evidence.OS = runtime.GOOS
		}
		if evidence.Architecture == "" {
			evidence.Architecture = runtime.GOARCH
		}
		if evidence.ObservedAt == "" {
			evidence.ObservedAt = "2026-08-21T00:00:00Z"
		}
	}
	if evidence.Artifact.Repository == "" {
		evidence.Artifact.Repository = "owner/evidence"
	}
	if evidence.Artifact.Revision == "" {
		evidence.Artifact.Revision = strings.Repeat("e", 40)
	}
	if evidence.Artifact.Path == "" {
		evidence.Artifact.Path = "evidence/result.json"
	}
	if evidence.Artifact.Digest == "" {
		evidence.Artifact.Digest = "sha256:" + strings.Repeat("f", 64)
	}
	if evidence.Level == "schema" || evidence.Level == "materialization" {
		evidence.Trust = &domain.DirectoryEvidenceTrust{
			Kind:         "github_actions",
			Workflow:     evidence.Artifact.Repository + "/.github/workflows/directory.yml",
			SourceRef:    "refs/heads/main",
			SourceDigest: evidence.Artifact.Revision,
		}
	} else {
		evidence.Trust = &domain.DirectoryEvidenceTrust{Kind: "reviewed_external"}
	}
	return evidence
}
