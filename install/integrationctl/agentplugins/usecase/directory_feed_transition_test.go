package usecase

import (
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func TestDirectoryTransitionCannotBorrowAnotherSignedFeed(t *testing.T) {
	revision := strings.Repeat("a", 40)
	current := domain.DirectoryOrigin{
		ProductID: "demo", DistributionID: "owner/demo",
		DistributionKind: domain.DistributionCommunity,
		DesiredReleaseSequence: 1, SnapshotSchema: 1,
		SnapshotSequence: 43, SnapshotDigest: "sha256:legacy",
	}
	installation := domain.Installation{
		OriginMode: domain.OriginModeDirectory,
		Directory: &current,
		Source: domain.SourceBinding{
			Repository: "owner/demo", ResolvedRevision: revision,
			TreeDigest: "sha256:tree",
		},
		Package: domain.PackageBinding{ManifestDigest: "sha256:manifest"},
	}
	input := AddInput{
		OriginMode: domain.OriginModeDirectory,
		DirectoryResolution: &domain.DirectoryOrigin{
			ProductID: current.ProductID, DistributionID: current.DistributionID,
			DistributionKind: current.DistributionKind,
			DesiredReleaseSequence: current.DesiredReleaseSequence,
			SnapshotSchema: 1, SnapshotSequence: 5, SnapshotDigest: "sha256:modern",
		},
		Envelope: domain.PackageEnvelope{
			Source: domain.SourceIdentity{Repository: "owner/demo", ResolvedRevision: revision},
			TreeDigest: "sha256:tree", ManifestDigest: "sha256:manifest",
		},
	}
	cases := []struct {
		name, currentFeed, incomingFeed string
		reject bool
	}{
		{"legacy-to-modern", "", "modern-v1", true},
		{"modern-to-legacy", "modern-v1", "", true},
		{"modern-to-other", "modern-v1", "other-v1", true},
		{"legacy-to-legacy", "", "", false},
		{"modern-to-same-modern", "modern-v1", "modern-v1", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			installation.Directory.FeedID = tc.currentFeed
			input.DirectoryResolution.FeedID = tc.incomingFeed
			err := validateDirectoryTransition(installation, input)
			if tc.reject && (err == nil || !strings.Contains(err.Error(), "feed change requires switch")) {
				t.Fatalf("cross-feed transition was accepted: %v", err)
			}
			if !tc.reject && err != nil {
				t.Fatalf("same-feed transition was rejected: %v", err)
			}
		})
	}
}
