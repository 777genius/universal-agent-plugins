package securityscan

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

const maxPublishedFindings = 32

type Evaluator struct {
	Scanner     Scanner
	Cache       Cache
	Requirement domain.SecurityRequirement
}

func (evaluator Evaluator) Evaluate(ctx context.Context, input domain.SecurityEvaluationInput) (domain.SecurityAssessment, error) {
	requirement := evaluator.Requirement
	if requirement.Scanner.ID == "" {
		requirement = DefaultRequirement()
	}
	subject := domain.SecuritySubject{TreeDigest: input.TreeDigest, ManifestDigest: input.ManifestDigest}
	if input.Trusted != nil {
		trusted := *input.Trusted
		if err := trusted.Validate(requirement, subject); err == nil {
			trusted.Evidence = domain.SecurityEvidenceSignedIndex
			return trusted, nil
		}
	}
	key := CacheKey(subject, requirement)
	if evaluator.Cache != nil {
		if cached, ok := evaluator.Cache.Load(key); ok && cached.Validate(requirement, subject) == nil {
			cached.Evidence = domain.SecurityEvidenceCache
			return cached, nil
		}
	}
	if evaluator.Scanner == nil {
		return domain.SecurityAssessment{}, errors.New("security scanner is unavailable")
	}
	body, err := evaluator.Scanner.Scan(ctx, input.SnapshotRoot)
	if err != nil {
		return domain.SecurityAssessment{}, err
	}
	report, err := decodeReport(body)
	if err != nil {
		return domain.SecurityAssessment{}, err
	}
	assessment, err := assessmentFromReport(subject, requirement, report, body)
	if err != nil {
		return domain.SecurityAssessment{}, err
	}
	assessment.Evidence = domain.SecurityEvidenceLocalScan
	if evaluator.Cache != nil {
		cached := assessment
		cached.Evidence = ""
		if err := evaluator.Cache.Store(key, cached); err != nil {
			return domain.SecurityAssessment{}, fmt.Errorf("store security assessment: %w", err)
		}
	}
	return assessment, nil
}

func assessmentFromReport(subject domain.SecuritySubject, requirement domain.SecurityRequirement, report rawReport, rawReportBody []byte) (domain.SecurityAssessment, error) {
	digest := sha256.Sum256(rawReportBody)
	assessment := domain.SecurityAssessment{
		SchemaVersion: domain.SecurityReportSchemaVersion,
		Subject:       subject,
		Scanner:       requirement.Scanner,
		Policy:        requirement.Policy,
		ScannedFiles:  report.Stats.ScannedFiles,
		ReportDigest:  "sha256:" + hex.EncodeToString(digest[:]),
		Findings:      make([]domain.SecurityFinding, 0, min(len(report.Findings), maxPublishedFindings)),
	}
	for _, finding := range report.Findings {
		code := strings.TrimSpace(finding.RuleCode)
		message := strings.TrimSpace(finding.Message)
		if code == "" || message == "" {
			return domain.SecurityAssessment{}, errors.New("lintai report contains an incomplete finding")
		}
		disposition := disposition(code, strings.ToLower(finding.Severity), strings.ToLower(finding.Confidence))
		assessment.Counts.Total++
		if disposition == "blocking" {
			assessment.Counts.Blocking++
		} else {
			assessment.Counts.Warnings++
		}
		line := 0
		if finding.Location.Start != nil {
			line = finding.Location.Start.Line
		}
		assessment.Findings = append(assessment.Findings, domain.SecurityFinding{
			Code: code, Disposition: disposition, Severity: strings.ToLower(finding.Severity), Confidence: strings.ToLower(finding.Confidence),
			Category: strings.ToLower(finding.Category), Path: filepathSlash(strings.TrimSpace(finding.Location.NormalizedPath)), Line: line, Message: message,
		})
	}
	domain.SortSecurityFindings(assessment.Findings)
	if len(assessment.Findings) > maxPublishedFindings {
		assessment.Findings = assessment.Findings[:maxPublishedFindings]
	}
	assessment.Outcome = domain.SecurityNoBlockingFindings
	if assessment.Counts.Blocking > 0 {
		assessment.Outcome = domain.SecurityBlockingFindings
	} else if assessment.Counts.Warnings > 0 {
		assessment.Outcome = domain.SecurityWarnings
	}
	if err := assessment.Validate(requirement, subject); err != nil {
		return domain.SecurityAssessment{}, err
	}
	return assessment, nil
}

func filepathSlash(path string) string {
	return strings.ReplaceAll(path, "\\", "/")
}
