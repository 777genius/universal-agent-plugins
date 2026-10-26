//go:build !windows

package securityscan

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// This expressly inert script exercises the actual process capture, argv and
// cwd contract. It is never used as native evidence or a scanner substitute.
func commandScannerProcessFixture(t *testing.T, stdout, stderr string, exit int) (CommandScanner, string) {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	quote := func(value string) string { return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'" }
	program := "#!/bin/sh\n" +
		"if [ \"$#\" -ne 2 ] || [ \"$1\" != scan-agent-plugin ] || [ \"$2\" != \"$PWD\" ]; then exit 9; fi\n" +
		"printf '%s' " + quote(stdout) + "\nprintf '%s' " + quote(stderr) + " >&2\nexit " + strconv.Itoa(exit) + "\n"
	executable := filepath.Join(root, "expressly-inert-scanner")
	if err := os.WriteFile(executable, []byte(program), 0700); err != nil {
		t.Fatal(err)
	}
	return CommandScanner{Executable: executable}, root
}

func TestCommandScannerPreservesValidExitZeroAndOneReports(t *testing.T) {
	report := reportJSON(t, []map[string]any{finding("SEC330", "warn", "high", "mcp.json", "inert selected finding")})
	for _, exit := range []int{0, 1} {
		t.Run(strconv.Itoa(exit), func(t *testing.T) {
			scanner, root := commandScannerProcessFixture(t, string(report), "ignored valid-report diagnostics\n", exit)
			body, err := scanner.Scan(context.Background(), root)
			if err != nil || !bytes.Equal(body, report) {
				t.Fatalf("valid exit %d report changed: %v", exit, err)
			}
			assessment, err := (Evaluator{Scanner: scanner, Requirement: DefaultRequirement()}).Evaluate(context.Background(), domain.SecurityEvaluationInput{
				SnapshotRoot: root, TreeDigest: testTreeDigest, ManifestDigest: testManifestDigest,
			})
			if err != nil || assessment.Outcome != domain.SecurityBlockingFindings || assessment.Counts.Blocking != 1 || assessment.Evidence != domain.SecurityEvidenceLocalScan {
				t.Fatalf("valid exit %d report bypassed or failed assessment: %+v (%v)", exit, assessment, err)
			}
		})
	}
}

func TestCommandScannerExitOneInvalidJSONRetainsOnlyBoundedFailureDiagnostics(t *testing.T) {
	for _, stdout := range []string{"", "private non-JSON stdout must stay private"} {
		t.Run(strconv.Itoa(len(stdout)), func(t *testing.T) {
			stderr := "  inert-loader: version GLIBC_REQUIRED not found\n"
			scanner, root := commandScannerProcessFixture(t, stdout, stderr, 1)
			body, err := scanner.Scan(context.Background(), root)
			want := fmt.Sprintf("lintai security scan failed (exit 1, invalid JSON, %d stdout bytes): %s", len(stdout), strings.TrimSpace(stderr))
			if body != nil || err == nil || err.Error() != want || (stdout != "" && strings.Contains(err.Error(), stdout)) {
				t.Fatalf("exit-one diagnostic: body=%q err=%v, want=%q", body, err, want)
			}
		})
	}
}

func TestCommandScannerValidJSONStillRequiresIndependentReportIdentity(t *testing.T) {
	scanner, root := commandScannerProcessFixture(t, "{}", "", 1)
	body, err := scanner.Scan(context.Background(), root)
	if err != nil || string(body) != "{}" {
		t.Fatalf("syntax-only scan behavior changed: %q (%v)", body, err)
	}
	_, err = (Evaluator{Scanner: scanner, Requirement: DefaultRequirement()}).Evaluate(context.Background(), domain.SecurityEvaluationInput{
		SnapshotRoot: root, TreeDigest: testTreeDigest, ManifestDigest: testManifestDigest,
	})
	if err == nil || err.Error() != "lintai report has an unsupported scanner or policy identity" {
		t.Fatalf("unsupported exit-one report identity admitted: %v", err)
	}
}

func TestCommandScannerPreservesExitZeroInvalidJSONAndOtherExitFailures(t *testing.T) {
	scanner, root := commandScannerProcessFixture(t, "not-json", "exit-zero diagnostics\n", 0)
	body, err := scanner.Scan(context.Background(), root)
	if err != nil || string(body) != "not-json" {
		t.Fatalf("exit-zero capture changed: %q (%v)", body, err)
	}
	if _, err := decodeReport(body); err == nil || err.Error() != "lintai returned invalid JSON" {
		t.Fatalf("exit-zero invalid JSON was not independently refused: %v", err)
	}
	scanner, root = commandScannerProcessFixture(t, "private stdout", "  original other-exit failure\n", 2)
	body, err = scanner.Scan(context.Background(), root)
	if body != nil || err == nil || err.Error() != "lintai security scan failed: original other-exit failure" {
		t.Fatalf("other-exit failure changed: %q (%v)", body, err)
	}
}

func TestCommandScannerStreamOverflowPrecedesExitDiagnostics(t *testing.T) {
	for _, stream := range []string{"stdout", "stderr"} {
		for _, exit := range []int{0, 1} {
			t.Run(stream+"/"+strconv.Itoa(exit), func(t *testing.T) {
				stdout, stderr := "not-json", ""
				if stream == "stdout" {
					stdout = strings.Repeat("x", maxScannerOutput+1)
				} else {
					stderr = strings.Repeat("x", maxScannerError+1)
				}
				scanner, root := commandScannerProcessFixture(t, stdout, stderr, exit)
				body, err := scanner.Scan(context.Background(), root)
				if body != nil || err == nil || err.Error() != "lintai security scan exceeded its output limit" {
					t.Fatalf("stream overflow refused incorrectly: successful body bytes=%d, error is nil=%t", len(body), err == nil)
				}
			})
		}
	}
}
