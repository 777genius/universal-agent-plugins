package sourceacquisition

import (
	"context"
	"errors"
	"fmt"
	"testing"

	processadapter "github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/process"
	legacyports "github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

func TestGitCleanupEvidencePreservesDiagnosticsAndClosure(t *testing.T) {
	for _, tc := range []struct {
		name   string
		result legacyports.CommandResult
		cause  error
		unsafe bool
	}{
		{"clean success", legacyports.CommandResult{CleanupComplete: true}, nil, false},
		{"prelaunch cancellation", legacyports.CommandResult{NoChildStarted: true}, context.Canceled, false},
		{"confirmed cancellation", legacyports.CommandResult{CleanupComplete: true}, context.DeadlineExceeded, false},
		{"unconfirmed cancellation", legacyports.CommandResult{}, context.DeadlineExceeded, true},
		{"unconfirmed cleanup", legacyports.CommandResult{}, errors.New("exact original cleanup diagnostic"), true},
		{"unattested success", legacyports.CommandResult{}, nil, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := (OSRunner{}).withCleanupEvidence(tc.result, tc.cause); err != tc.cause { //nolint:errorlint // The default CLI contract preserves exact error identity, including nil.
				t.Fatal("default CLI error contract changed", err)
			}
			err := (OSRunner{ReportCleanupEvidence: true}).withCleanupEvidence(tc.result, tc.cause)
			if errors.Is(err, ErrGitCleanupUnconfirmed) != tc.unsafe {
				t.Fatal("incorrect cleanup attestation", err)
			}
			if tc.cause != nil && (err.Error() != tc.cause.Error() || !errors.Is(err, tc.cause)) {
				t.Fatal("original diagnostic or cause changed", err)
			}
			if tc.unsafe && !errors.Is(fmt.Errorf("existing wrapper: %w", err), ErrGitCleanupUnconfirmed) {
				t.Fatal("wrapped cleanup uncertainty was lost")
			}
		})
	}
}

func TestGitCleanupEvidenceKeepsDefaultStdoutLimitClassification(t *testing.T) {
	result := legacyports.CommandResult{}
	cause := processadapter.ErrStdoutLimitExceeded
	legacy := (OSRunner{}).withCleanupEvidence(result, cause)
	if legacy != cause || !processadapter.IsOnlyStdoutLimitExceeded(legacy) { //nolint:errorlint // Adding even a wrapper would change this legacy classification contract.
		t.Fatal("default stdout limit classification changed", legacy)
	}
	hosted := (OSRunner{ReportCleanupEvidence: true}).withCleanupEvidence(result, cause)
	if !errors.Is(hosted, cause) || !errors.Is(hosted, ErrGitCleanupUnconfirmed) || processadapter.IsOnlyStdoutLimitExceeded(hosted) {
		t.Fatal("hosted stdout limit lost cleanup uncertainty", hosted)
	}
}
