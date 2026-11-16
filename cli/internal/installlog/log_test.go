package installlog

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestWriteRecordsRunAndEntriesPrivately(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "logs")
	run := Run{Command: "skills add", Version: "1.2.3", Scope: "user", OperationID: "op-1", Attempt: 2, Entries: []Entry{
		{Alias: "pdf", Client: "codex", Status: "failed", Code: "TARGET_UNAVAILABLE", Summary: "agent not verified", Detail: "prepare pdf/codex: boom"},
		{Alias: "pdf", Client: "claude", Status: "completed"},
	}}
	path, err := Write(dir, run, time.Date(2026, 11, 15, 12, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(filepath.Base(path), "install-20261115T120000Z-") {
		t.Fatalf("unexpected name %s", path)
	}
	if runtime.GOOS != "windows" {
		if info, _ := os.Stat(path); info.Mode().Perm() != 0o600 {
			t.Fatalf("log must be private, got %v", info.Mode().Perm())
		}
		if info, _ := os.Stat(dir); info.Mode().Perm() != 0o700 {
			t.Fatalf("log directory must be private, got %v", info.Mode().Perm())
		}
	}
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = file.Close() })
	var lines []map[string]any
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		var line map[string]any
		if err := json.Unmarshal(scanner.Bytes(), &line); err != nil {
			t.Fatal(err)
		}
		lines = append(lines, line)
	}
	if len(lines) != 3 || lines[0]["type"] != "run" || lines[0]["attempt"] != float64(2) || lines[1]["detail"] != "prepare pdf/codex: boom" || lines[2]["status"] != "completed" {
		t.Fatalf("unexpected records %v", lines)
	}
}

func TestWriteDisabledWithoutDirectory(t *testing.T) {
	if path, err := Write("", Run{Command: "x"}, time.Now()); path != "" || err != nil {
		t.Fatalf("empty directory must disable logging: %q %v", path, err)
	}
}

func TestWriteKeepsOnlyRecentFilesAndNeverTouchesForeignOnes(t *testing.T) {
	dir := t.TempDir()
	foreign := filepath.Join(dir, "notes.txt")
	if err := os.WriteFile(foreign, []byte("mine"), 0o600); err != nil {
		t.Fatal(err)
	}
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	var first string
	for i := range maxFiles + 5 {
		path, err := Write(dir, Run{Command: "add"}, base.Add(time.Duration(i)*time.Minute))
		if err != nil {
			t.Fatal(err)
		}
		if i == 0 {
			first = path
		}
	}
	entries, _ := os.ReadDir(dir)
	logs := 0
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), filePrefix) {
			logs++
		}
	}
	if logs != maxFiles {
		t.Fatalf("want %d retained logs, got %d", maxFiles, logs)
	}
	if _, err := os.Stat(first); !os.IsNotExist(err) {
		t.Fatal("the oldest log must be pruned")
	}
	if _, err := os.Stat(foreign); err != nil {
		t.Fatal("a file this package did not create must never be removed")
	}
}

func TestSanitizeRemovesCredentialsButKeepsPaths(t *testing.T) {
	in := "clone https://user:pa55@github.com/acme/tools failed in /Users/me/.agents; token=ghp_abcdef123456 Authorization: Bearer abcdefghijklmnop api_key: sk-123"
	got := Sanitize(in)
	for _, secret := range []string{"pa55", "ghp_abcdef123456", "abcdefghijklmnop", "sk-123"} {
		if strings.Contains(got, secret) {
			t.Fatalf("secret %q leaked: %s", secret, got)
		}
	}
	if !strings.Contains(got, "/Users/me/.agents") || !strings.Contains(got, "github.com/acme/tools") {
		t.Fatalf("diagnostic paths must survive: %s", got)
	}
}

func TestFailedDetectsNonSuccessStatuses(t *testing.T) {
	if (Run{Entries: []Entry{{Status: "completed"}, {Status: "unchanged"}}}).Failed() {
		t.Fatal("a clean run is not a failure")
	}
	if !(Run{Entries: []Entry{{Status: "completed"}, {Status: "not_attempted"}}}).Failed() {
		t.Fatal("an unattempted binding is worth logging")
	}
}

func TestSanitizeCoversCommonCredentialShapes(t *testing.T) {
	secrets := []string{"sk-ant-abcdefghijklmnop1234", "client-secret-value", "MySvcTokenValue123", "ghp_abcdefghijklmnopqrstuv1234", "plain-password-1", "flagtokenvalue99", "xoxb-1234567890-abcdef"}
	cases := []string{
		"ANTHROPIC_API_KEY=sk-ant-abcdefghijklmnop1234 failed",
		`config {"client_secret": "client-secret-value"} rejected`,
		"MY_SERVICE_TOKEN=MySvcTokenValue123",
		"Authorization: token ghp_abcdefghijklmnopqrstuv1234",
		"password: plain-password-1",
		"run --api-key flagtokenvalue99 --verbose",
		"slack xoxb-1234567890-abcdef leaked",
	}
	for index, in := range cases {
		got := Sanitize(in)
		if strings.Contains(got, secrets[index]) {
			t.Fatalf("case %d leaked %q: %s", index, secrets[index], got)
		}
	}
	// Ordinary diagnostics must survive redaction.
	kept := "prepare pdf/codex: target codex is not detected; nothing was changed at /Users/me/.codex (author=alice)"
	if got := Sanitize(kept); got != kept {
		t.Fatalf("redaction altered a harmless message:\n%s", got)
	}
}
