package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestInspectReportsIgnoredV1ExtensionsAndNeverExecutesComponents(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "plugin.json"), `{"$schema":"https://agent-plugins.org/schemas/1.0.0/plugin.schema.json","name":"fixture","unknown":true,"extensions":[]}`)
	writeFile(t, filepath.Join(root, "mcp.json"), `{"$schema":"https://agent-plugins.org/schemas/1.0.0/mcp.schema.json","mcpServers":{"docs":{"type":"stdio","command":"definitely-not-executed"}}}`)

	report, err := inspect(root)
	if err != nil {
		t.Fatal(err)
	}
	if report.Rejected != nil || len(report.Loaded.MCPServers) != 1 {
		t.Fatalf("report = %+v", report)
	}
	if got := []string{report.Reported[0].Field, report.Reported[1].Field}; got[0] != "extensions" || got[1] != "unknown" {
		t.Fatalf("reported = %+v", report.Reported)
	}
}

func TestRunEmitsProtocolReport(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "plugin.json"), `{"$schema":"https://agent-plugins.org/schemas/1.0.0/plugin.schema.json","name":"fixture"}`)
	var stdout, stderr bytes.Buffer
	if code := run([]string{root}, &stdout, &stderr); code != 0 {
		t.Fatalf("exit = %d, stderr = %s", code, stderr.String())
	}
	var report loadReport
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatalf("decode report: %v", err)
	}
	if report.Rejected != nil || report.Loaded.Skills == nil || report.Skipped == nil || report.Reported == nil {
		t.Fatalf("protocol report = %+v", report)
	}
}

func writeFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}
