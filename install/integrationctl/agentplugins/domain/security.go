package domain

import (
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"
)

const (
	SecurityReportSchemaVersion = 1
	SecurityScannerID           = "lintai"
	SecurityScannerVersion      = "0.1.3"
	SecurityPolicyID            = "agent-plugin-install"
	SecurityPolicyVersion       = 2
)

var securityDigestPattern = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)

type SecurityOutcome string

const (
	SecurityNoBlockingFindings SecurityOutcome = "no_blocking_findings"
	SecurityWarnings           SecurityOutcome = "warnings"
	SecurityBlockingFindings   SecurityOutcome = "blocking_findings"
)

type SecurityEvidenceSource string

const (
	SecurityEvidenceSignedIndex SecurityEvidenceSource = "signed_index"
	SecurityEvidenceCache       SecurityEvidenceSource = "cache"
	SecurityEvidenceLocalScan   SecurityEvidenceSource = "local_scan"
)

type SecuritySubject struct {
	TreeDigest     string `json:"tree_digest"`
	ManifestDigest string `json:"manifest_digest"`
}

type SecurityScanner struct {
	ID      string `json:"id"`
	Version string `json:"version"`
}

type SecurityPolicy struct {
	ID      string `json:"id"`
	Version int    `json:"version"`
	Digest  string `json:"digest"`
}

type SecurityCounts struct {
	Blocking int `json:"blocking"`
	Warnings int `json:"warnings"`
	Total    int `json:"total"`
}

type SecurityFinding struct {
	Code        string `json:"code"`
	Disposition string `json:"disposition"`
	Severity    string `json:"severity"`
	Confidence  string `json:"confidence"`
	Category    string `json:"category"`
	Path        string `json:"path"`
	Line        int    `json:"line,omitempty"`
	Message     string `json:"message"`
}

type SecurityAssessment struct {
	SchemaVersion int                    `json:"schema_version"`
	Subject       SecuritySubject        `json:"subject"`
	Scanner       SecurityScanner        `json:"scanner"`
	Policy        SecurityPolicy         `json:"policy"`
	Outcome       SecurityOutcome        `json:"outcome"`
	Counts        SecurityCounts         `json:"counts"`
	ScannedFiles  int                    `json:"scanned_files"`
	ReportDigest  string                 `json:"report_digest"`
	Findings      []SecurityFinding      `json:"findings,omitempty"`
	Evidence      SecurityEvidenceSource `json:"evidence_source,omitempty"`
}

type SecurityRequirement struct {
	Scanner SecurityScanner
	Policy  SecurityPolicy
}

type SecurityEvaluationInput struct {
	SnapshotRoot   string
	TreeDigest     string
	ManifestDigest string
	Trusted        *SecurityAssessment
}

func (assessment SecurityAssessment) Validate(requirement SecurityRequirement, subject SecuritySubject) error {
	if assessment.SchemaVersion != SecurityReportSchemaVersion {
		return fmt.Errorf("unsupported security report schema %d", assessment.SchemaVersion)
	}
	if assessment.Subject != subject {
		return errors.New("security assessment does not describe the acquired package")
	}
	if assessment.Scanner != requirement.Scanner || assessment.Policy != requirement.Policy {
		return errors.New("security assessment scanner or policy is stale")
	}
	if !securityDigestPattern.MatchString(assessment.Subject.TreeDigest) || !securityDigestPattern.MatchString(assessment.Subject.ManifestDigest) || !securityDigestPattern.MatchString(assessment.Policy.Digest) || !securityDigestPattern.MatchString(assessment.ReportDigest) {
		return errors.New("security assessment contains an invalid digest")
	}
	if assessment.Counts.Blocking < 0 || assessment.Counts.Warnings < 0 || assessment.Counts.Total < 0 || assessment.Counts.Total != assessment.Counts.Blocking+assessment.Counts.Warnings || assessment.ScannedFiles < 0 {
		return errors.New("security assessment contains invalid counts")
	}
	expected := SecurityNoBlockingFindings
	if assessment.Counts.Blocking > 0 {
		expected = SecurityBlockingFindings
	} else if assessment.Counts.Warnings > 0 {
		expected = SecurityWarnings
	}
	if assessment.Outcome != expected {
		return errors.New("security assessment outcome does not match its counts")
	}
	projectedBlocking, projectedWarnings := 0, 0
	for _, finding := range assessment.Findings {
		if strings.TrimSpace(finding.Code) == "" || strings.TrimSpace(finding.Message) == "" || (finding.Disposition != "blocking" && finding.Disposition != "warning") {
			return errors.New("security assessment contains an invalid finding")
		}
		if finding.Disposition == "blocking" {
			projectedBlocking++
		} else {
			projectedWarnings++
		}
	}
	if projectedBlocking > assessment.Counts.Blocking || projectedWarnings > assessment.Counts.Warnings {
		return errors.New("security assessment finding projection exceeds its counts")
	}
	return nil
}

func SortSecurityFindings(findings []SecurityFinding) {
	sort.Slice(findings, func(i, j int) bool {
		left, right := findings[i], findings[j]
		if left.Disposition != right.Disposition {
			return left.Disposition < right.Disposition
		}
		if left.Code != right.Code {
			return left.Code < right.Code
		}
		if left.Path != right.Path {
			return left.Path < right.Path
		}
		if left.Line != right.Line {
			return left.Line < right.Line
		}
		return left.Message < right.Message
	})
}
