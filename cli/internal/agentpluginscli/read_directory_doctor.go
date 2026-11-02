package agentpluginscli

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func doctorDirectorySafetyFindings(ctx context.Context, app App, state domain.StateFileV2, selected *domain.Installation, selectedView *publicInstallation) ([]doctorFinding, error) {
	type item struct {
		installation domain.Installation
		warnings     []publicSafetyWarning
	}
	items := []item{}
	if selected != nil && selectedView != nil {
		items = append(items, item{installation: *selected, warnings: selectedView.Warnings})
	} else {
		for _, installation := range state.Installations {
			view, err := inspectInstalledProduct(ctx, app, state, installation)
			if err != nil {
				return nil, err
			}
			items = append(items, item{installation: installation, warnings: view.Warnings})
		}
	}
	findings := []doctorFinding{}
	for _, entry := range items {
		for _, warning := range entry.warnings {
			clientID := ""
			if len(warning.Clients) == 1 {
				clientID = warning.Clients[0]
			}
			findings = append(findings, doctorFinding{Status: "degraded", Code: warning.Code, ClientID: clientID,
				InstallationName: entry.installation.DeclaredName, InstallationID: entry.installation.InstallationID,
				Message: warning.Message, RecoveryAction: warning.Action})
		}
	}
	return findings, nil
}

func renderProductInspection(writer io.Writer, product publicProductInspection) error {
	_, _ = fmt.Fprintf(writer, "%s (%s)\n", product.DisplayName, product.ProductID)
	_, _ = fmt.Fprintf(writer, "  Reviewed default: %s\n", product.ReviewedDefault)
	_, _ = fmt.Fprintf(writer, "  Selected: %s (%s)\n", product.SelectedDistribution, product.SelectionReason)
	_, _ = fmt.Fprintf(writer, "  Release: sequence=%d source=%s@%s//%s tree=%s manifest=%s\n", product.SelectedRelease.ReleaseSequence,
		product.SelectedRelease.Repository, product.SelectedRelease.ResolvedRevision, product.SelectedRelease.PackagePath,
		product.SelectedRelease.TreeDigest, product.SelectedRelease.ManifestDigest)
	for _, target := range product.TargetCompatibility {
		_, _ = fmt.Fprintf(writer, "  %s: delivery=%s evidence=%s\n", target.Client, target.Delivery, strings.Join(target.Evidence, ","))
	}
	for _, alternative := range product.Alternatives {
		_, _ = fmt.Fprintf(writer, "  Alternative: %s kind=%s status=%s release=%d\n", alternative.ID, alternative.Kind, alternative.Status, alternative.ReleaseSequence)
	}
	for _, evidence := range product.ImmutableEvidence {
		_, _ = fmt.Fprintf(writer, "  Evidence: %s %s=%s trust=%s %s@%s//%s digest=%s\n", evidence.ID, evidence.Level, evidence.Outcome, evidenceTrustLabel(evidence),
			evidence.Artifact.Repository, evidence.Artifact.Revision, evidence.Artifact.Path, evidence.Artifact.Digest)
	}
	return nil
}

func evidenceTrustLabel(evidence publicEvidence) string {
	record := domain.DirectoryEvidence{Artifact: evidence.Artifact, Trust: evidence.Trust}
	if record.HasTrustedProvenance() {
		return "trusted"
	}
	return "self-reported/informational"
}
