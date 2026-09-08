package agentpluginscli

import (
	"fmt"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/spf13/cobra"
)

const securityFindingPreviewLimit = 3

var repositoryMaintenanceSecurityCodes = map[string]struct{}{
	"SEC324": {},
	"SEC325": {},
	"SEC326": {},
	"SEC327": {},
	"SEC328": {},
}

func authorizeSecurityAssessment(cmd *cobra.Command, app App, opts *options, loaded *loadedPackage) error {
	if loaded.security == nil || loaded.securityAuthorized {
		return nil
	}
	assessment := *loaded.security
	if opts.format == "human" {
		renderSecurityAssessment(cmd, assessment, opts.securityDetails)
	}
	if assessment.Counts.Blocking == 0 || opts.dryRun {
		loaded.securityAuthorized = true
		return nil
	}
	if opts.acceptSecurityRisk {
		if opts.format == "human" {
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), "Continuing because --accept-security-risk was explicitly provided.")
		}
		loaded.securityAuthorized = true
		return nil
	}
	if opts.format != "human" || !app.Terminal {
		if opts.format == "json" {
			if err := writeJSONResult(cmd.OutOrStdout(), cmd.Name(), outputResultFailure, map[string]any{
				"status": "blocked_by_security_review", "security": assessment,
				"next_action": "Review the findings and rerun with --accept-security-risk only if you accept the risk.",
			}); err != nil {
				return err
			}
		}
		return fmt.Errorf("installation stopped: automated security checks found %d blocking finding(s); review them and rerun with --accept-security-risk to continue", assessment.Counts.Blocking)
	}
	confirmed, err := promptYesNo(cmd.InOrStdin(), cmd.OutOrStdout(), "Blocking security findings were detected. Continue anyway? [y/N]")
	if err != nil {
		return err
	}
	if !confirmed {
		return fmt.Errorf("installation cancelled after automated security review; no files were changed")
	}
	loaded.securityAuthorized = true
	return nil
}

func renderSecurityAssessment(cmd *cobra.Command, assessment domain.SecurityAssessment, showAll bool) {
	source := "local scan"
	switch assessment.Evidence {
	case domain.SecurityEvidenceSignedIndex:
		source = "signed index"
	case domain.SecurityEvidenceCache:
		source = "verified cache"
	}
	installNotes, maintenanceNotes := splitSecurityFindings(assessment.Findings)
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Automated review: %s", securityOutcomeLabel(assessment))
	if assessment.Counts.Blocking == 0 && assessment.Counts.Warnings > 0 && assessment.Counts.Warnings == len(assessment.Findings) {
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), " (%s)", securityNoteBreakdown(len(installNotes), len(maintenanceNotes)))
	}
	_, _ = fmt.Fprintln(cmd.OutOrStdout(), ".")

	visible := visibleSecurityFindings(assessment, installNotes, maintenanceNotes, showAll)
	for _, finding := range visible {
		location := strings.TrimSpace(finding.Path)
		if finding.Line > 0 {
			location = fmt.Sprintf("%s:%d", location, finding.Line)
		}
		if location != "" {
			location += " - "
		}
		label := "NOTE"
		if finding.Disposition == "blocking" {
			label = "BLOCKING"
		}
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  [%s] %s %s%s\n", label, finding.Code, location, finding.Message)
	}
	visibleBlocking, visibleWarnings := findingDispositionCounts(visible)
	if hiddenBlocking := assessment.Counts.Blocking - visibleBlocking; hiddenBlocking > 0 {
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%d additional blocking %s are outside the bounded finding preview.\n", hiddenBlocking, plural(hiddenBlocking, "finding", "findings"))
	}
	if hiddenWarnings := assessment.Counts.Warnings - visibleWarnings; hiddenWarnings > 0 {
		if showAll {
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%d additional %s are outside the bounded finding preview.\n", hiddenWarnings, plural(hiddenWarnings, "note", "notes"))
		} else {
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%d additional %s hidden. Rerun with --security-details to review all published details.\n", hiddenWarnings, plural(hiddenWarnings, "note", "notes"))
		}
	}
	if showAll || assessment.Counts.Blocking > 0 {
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Evidence: %s for the exact package revision. Automated checks are not a guarantee of safety.\n", source)
	}
}

func findingDispositionCounts(findings []domain.SecurityFinding) (blocking, warnings int) {
	for _, finding := range findings {
		if finding.Disposition == "blocking" {
			blocking++
		} else {
			warnings++
		}
	}
	return blocking, warnings
}

func splitSecurityFindings(findings []domain.SecurityFinding) (install, maintenance []domain.SecurityFinding) {
	for _, finding := range findings {
		if _, ok := repositoryMaintenanceSecurityCodes[finding.Code]; ok {
			maintenance = append(maintenance, finding)
			continue
		}
		install = append(install, finding)
	}
	return install, maintenance
}

func visibleSecurityFindings(assessment domain.SecurityAssessment, install, maintenance []domain.SecurityFinding, showAll bool) []domain.SecurityFinding {
	if showAll {
		return append(append([]domain.SecurityFinding(nil), install...), maintenance...)
	}
	visible := make([]domain.SecurityFinding, 0, securityFindingPreviewLimit)
	for _, finding := range install {
		if finding.Disposition == "blocking" {
			visible = append(visible, finding)
		}
	}
	if assessment.Counts.Blocking > 0 {
		return visible
	}
	for _, finding := range install {
		if len(visible) == securityFindingPreviewLimit {
			break
		}
		visible = append(visible, finding)
	}
	return visible
}

func securityNoteBreakdown(install, maintenance int) string {
	parts := make([]string, 0, 2)
	if install > 0 {
		parts = append(parts, fmt.Sprintf("%d install %s", install, plural(install, "note", "notes")))
	}
	if maintenance > 0 {
		parts = append(parts, fmt.Sprintf("%d repository-maintenance %s", maintenance, plural(maintenance, "note", "notes")))
	}
	return strings.Join(parts, ", ")
}

func plural(count int, singular, pluralForm string) string {
	if count == 1 {
		return singular
	}
	return pluralForm
}

func securityOutcomeLabel(assessment domain.SecurityAssessment) string {
	if assessment.Counts.Blocking > 0 {
		return fmt.Sprintf("%d blocking %s and %d %s", assessment.Counts.Blocking, plural(assessment.Counts.Blocking, "finding", "findings"), assessment.Counts.Warnings, plural(assessment.Counts.Warnings, "note", "notes"))
	}
	if assessment.Counts.Warnings > 0 {
		return fmt.Sprintf("no blocking findings; %d %s found", assessment.Counts.Warnings, plural(assessment.Counts.Warnings, "note", "notes"))
	}
	return "no blocking findings"
}
