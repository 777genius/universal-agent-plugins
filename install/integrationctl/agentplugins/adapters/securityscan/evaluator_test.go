package securityscan

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

const (
	testTreeDigest     = "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	testManifestDigest = "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
)

type scannerStub struct {
	body  []byte
	calls int
}

func TestReleaseScannerVerifiesArchiveBeforeInstallingBinary(t *testing.T) {
	archive := tarArchive(t, "release/lintai", []byte("scanner"))
	digest := sha256.Sum256(archive)
	original := releaseAssets["test/test"]
	releaseAssets["test/test"] = releaseAsset{Name: "lintai.tar.gz", Digest: hex.EncodeToString(digest[:])}
	t.Cleanup(func() {
		if original.Name == "" {
			delete(releaseAssets, "test/test")
		} else {
			releaseAssets["test/test"] = original
		}
	})
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		_, _ = writer.Write(archive)
	}))
	defer server.Close()
	root := t.TempDir()
	resolved, err := (ReleaseScanner{Root: root, HTTPClient: server.Client(), Origin: server.URL + "/", GOOS: "test", GOARCH: "test"}).resolve(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(resolved)
	if err != nil || string(body) != "scanner" || filepath.Base(resolved) != "lintai" {
		t.Fatalf("unexpected installed scanner: path=%s body=%q err=%v", resolved, body, err)
	}
}

func TestReleaseScannerRejectsChecksumMismatch(t *testing.T) {
	original := releaseAssets["test/test"]
	releaseAssets["test/test"] = releaseAsset{Name: "lintai.tar.gz", Digest: strings.Repeat("0", 64)}
	t.Cleanup(func() {
		if original.Name == "" {
			delete(releaseAssets, "test/test")
		} else {
			releaseAssets["test/test"] = original
		}
	})
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		_, _ = writer.Write(tarArchive(t, "lintai", []byte("scanner")))
	}))
	defer server.Close()
	_, err := (ReleaseScanner{Root: t.TempDir(), HTTPClient: server.Client(), Origin: server.URL + "/", GOOS: "test", GOARCH: "test"}).resolve(context.Background())
	if err == nil || !strings.Contains(err.Error(), "checksum") {
		t.Fatalf("checksum mismatch error = %v", err)
	}
}

func tarArchive(t *testing.T, name string, body []byte) []byte {
	t.Helper()
	var output bytes.Buffer
	gz := gzip.NewWriter(&output)
	writer := tar.NewWriter(gz)
	if err := writer.WriteHeader(&tar.Header{Name: name, Typeflag: tar.TypeReg, Mode: 0o755, Size: int64(len(body))}); err != nil {
		t.Fatal(err)
	}
	if _, err := io.Copy(writer, bytes.NewReader(body)); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	return output.Bytes()
}

func (scanner *scannerStub) Scan(context.Context, string) ([]byte, error) {
	scanner.calls++
	return scanner.body, nil
}

type memoryCache map[string]domain.SecurityAssessment

func (cache memoryCache) Load(key string) (domain.SecurityAssessment, bool) {
	value, ok := cache[key]
	return value, ok
}

func (cache memoryCache) Store(key string, value domain.SecurityAssessment) error {
	cache[key] = value
	return nil
}

func TestEvaluatorClassifiesHighConfidenceInstallRiskAndCachesExactTuple(t *testing.T) {
	scanner := &scannerStub{body: reportJSON(t, []map[string]any{
		finding("SEC330", "warn", "high", "mcp.json", "remote content is piped to a shell"),
		finding("SEC302", "warn", "high", "mcp.json", "plain HTTP endpoint"),
	})}
	cache := memoryCache{}
	evaluator := Evaluator{Scanner: scanner, Cache: cache, Requirement: DefaultRequirement()}
	input := domain.SecurityEvaluationInput{SnapshotRoot: t.TempDir(), TreeDigest: testTreeDigest, ManifestDigest: testManifestDigest}

	first, err := evaluator.Evaluate(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	if first.Outcome != domain.SecurityBlockingFindings || first.Counts.Blocking != 1 || first.Counts.Warnings != 1 || first.Evidence != domain.SecurityEvidenceLocalScan {
		t.Fatalf("unexpected first assessment: %+v", first)
	}
	second, err := evaluator.Evaluate(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	if scanner.calls != 1 || second.Evidence != domain.SecurityEvidenceCache {
		t.Fatalf("exact cache tuple was not reused: calls=%d assessment=%+v", scanner.calls, second)
	}
}

func TestEvaluatorRejectsStaleTrustedEvidenceAndRescans(t *testing.T) {
	scanner := &scannerStub{body: reportJSON(t, nil)}
	requirement := DefaultRequirement()
	trusted := domain.SecurityAssessment{
		SchemaVersion: domain.SecurityReportSchemaVersion,
		Subject:       domain.SecuritySubject{TreeDigest: testTreeDigest, ManifestDigest: testManifestDigest},
		Scanner:       domain.SecurityScanner{ID: domain.SecurityScannerID, Version: "0.1.1"},
		Policy:        requirement.Policy,
		Outcome:       domain.SecurityNoBlockingFindings,
		ReportDigest:  testTreeDigest,
	}
	result, err := (Evaluator{Scanner: scanner, Requirement: requirement}).Evaluate(context.Background(), domain.SecurityEvaluationInput{
		SnapshotRoot: t.TempDir(), TreeDigest: testTreeDigest, ManifestDigest: testManifestDigest, Trusted: &trusted,
	})
	if err != nil {
		t.Fatal(err)
	}
	if scanner.calls != 1 || result.Evidence != domain.SecurityEvidenceLocalScan {
		t.Fatalf("stale evidence bypassed local scan: calls=%d result=%+v", scanner.calls, result)
	}
}

func TestEvaluatorAcceptsOnlyExactTrustedEvidenceWithoutScanning(t *testing.T) {
	requirement := DefaultRequirement()
	trusted := domain.SecurityAssessment{
		SchemaVersion: domain.SecurityReportSchemaVersion,
		Subject:       domain.SecuritySubject{TreeDigest: testTreeDigest, ManifestDigest: testManifestDigest},
		Scanner:       requirement.Scanner,
		Policy:        requirement.Policy,
		Outcome:       domain.SecurityWarnings,
		Counts:        domain.SecurityCounts{Warnings: 1, Total: 1},
		ScannedFiles:  2,
		ReportDigest:  testTreeDigest,
		Findings: []domain.SecurityFinding{{
			Code: "SEC301", Disposition: "warning", Severity: "warn", Confidence: "high",
			Category: "security", Path: "mcp.json", Message: "review endpoint",
		}},
	}
	scanner := &scannerStub{body: reportJSON(t, nil)}
	result, err := (Evaluator{Scanner: scanner, Requirement: requirement}).Evaluate(context.Background(), domain.SecurityEvaluationInput{
		SnapshotRoot: t.TempDir(), TreeDigest: testTreeDigest, ManifestDigest: testManifestDigest, Trusted: &trusted,
	})
	if err != nil {
		t.Fatal(err)
	}
	if scanner.calls != 0 || result.Evidence != domain.SecurityEvidenceSignedIndex || result.ReportDigest != trusted.ReportDigest {
		t.Fatalf("exact trusted evidence was not reused: calls=%d result=%+v", scanner.calls, result)
	}
}

func reportJSON(t *testing.T, findings []map[string]any) []byte {
	t.Helper()
	if findings == nil {
		findings = []map[string]any{}
	}
	body, err := json.Marshal(map[string]any{
		"schema_version": 1,
		"tool":           map[string]any{"name": "lintai", "version": domain.SecurityScannerVersion},
		"policy":         map[string]any{"id": domain.SecurityPolicyID, "version": domain.SecurityPolicyVersion, "presets": []string{"recommended", "preview", "threat-review", "supply-chain", "advisory"}},
		"stats":          map[string]any{"scanned_files": 3, "skipped_files": 0},
		"findings":       findings,
		"diagnostics":    []any{},
		"runtime_errors": []any{},
	})
	if err != nil {
		t.Fatal(err)
	}
	return body
}

func finding(code, severity, confidence, path, message string) map[string]any {
	return map[string]any{
		"rule_code": code, "category": "security", "severity": severity, "confidence": confidence, "message": message,
		"location": map[string]any{"normalized_path": path, "span": map[string]any{"start_byte": 0, "end_byte": 1}, "start": map[string]any{"line": 1, "column": 1}, "end": nil},
	}
}
