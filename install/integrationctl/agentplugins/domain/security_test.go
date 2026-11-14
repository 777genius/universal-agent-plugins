package domain

import (
	"strings"
	"testing"
)

func TestSecurityAssessmentRequiresExactCountsAndBoundedFindingProjection(t *testing.T) {
	subject := SecuritySubject{
		TreeDigest:     "sha256:" + strings.Repeat("a", 64),
		ManifestDigest: "sha256:" + strings.Repeat("b", 64),
	}
	requirement := SecurityRequirement{
		Scanner: SecurityScanner{ID: SecurityScannerID, Version: SecurityScannerVersion},
		Policy:  SecurityPolicy{ID: SecurityPolicyID, Version: SecurityPolicyVersion, Digest: "sha256:" + strings.Repeat("c", 64)},
	}
	valid := SecurityAssessment{
		SchemaVersion: SecurityReportSchemaVersion, Subject: subject, Scanner: requirement.Scanner, Policy: requirement.Policy,
		Outcome: SecurityWarnings, Counts: SecurityCounts{Warnings: 2, Total: 2}, ScannedFiles: 3,
		ReportDigest: "sha256:" + strings.Repeat("d", 64),
		Findings:     []SecurityFinding{{Code: "SEC301", Disposition: "warning", Message: "review endpoint"}},
	}
	if err := valid.Validate(requirement, subject); err != nil {
		t.Fatalf("valid projected assessment: %v", err)
	}
	invalidTotal := valid
	invalidTotal.Counts.Total = 3
	if err := invalidTotal.Validate(requirement, subject); err == nil {
		t.Fatal("accepted a count total that did not equal blocking plus warnings")
	}
	invalidProjection := valid
	invalidProjection.Counts = SecurityCounts{Warnings: 0, Total: 0}
	invalidProjection.Outcome = SecurityNoBlockingFindings
	if err := invalidProjection.Validate(requirement, subject); err == nil {
		t.Fatal("accepted a finding projection larger than the signed counts")
	}
}
