package pluginkitairepo_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"
)

const commandsTimeoutEntries = 32

// This is a post-return snapshot of persisted fixture metadata. It neither
// observes a live child nor changes cancellation, cleanup or the original fatal.
func commandsRecordTimeout(t *testing.T, root, label string, pid int, elapsed time.Duration, cause error) {
	t.Helper()
	body := commandsTimeoutSummary(root, label, pid, elapsed, cause)
	t.Logf("installer timeout persisted-state diagnostic (not a live process trace): %s", body)
	base, err := os.OpenRoot(root)
	if err != nil {
		t.Log("timeout diagnostic file unavailable")
		return
	}
	defer func() { _ = base.Close() }()
	name := commandsTimeoutToken(label) + "-installer-timeout.json"
	file, err := base.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		t.Log("timeout diagnostic file unavailable")
		return
	}
	_, writeErr := file.Write(append(body, '\n'))
	closeErr := file.Close()
	if writeErr != nil || closeErr != nil {
		t.Log("timeout diagnostic file incomplete")
	}
}

func commandsTimeoutSummary(root, label string, pid int, elapsed time.Duration, cause error) []byte {
	reason := "canceled"
	if errors.Is(cause, context.DeadlineExceeded) {
		reason = "deadline_exceeded"
	}
	report := map[string]any{"label": commandsTimeoutToken(label), "pid": pid,
		"elapsed_ms": elapsed.Milliseconds(), "reason": reason,
		"scope": "persisted fixture metadata only; live child and internal phase not inferred"}
	base, err := os.OpenRoot(root)
	if err != nil {
		report["root_status"] = "unavailable"
	} else {
		defer func() { _ = base.Close() }()
		report["state"] = commandsTimeoutState(base)
		report["journals"] = commandsTimeoutJournals(base)
		report["scratch"] = commandsTimeoutNames(base, "tmp")
		report["logs"] = commandsTimeoutNames(base, filepath.Join("installer-state", "logs"))
		locks := map[string]any{}
		for _, name := range []string{"environment.lock", "mutation.lock"} {
			var lock struct {
				PID        int    `json:"pid"`
				AcquiredAt string `json:"acquired_at"`
			}
			status := commandsTimeoutRead(base, filepath.Join("installer-state", name), &lock)
			if status != "read" {
				lock.PID, lock.AcquiredAt = 0, ""
			}
			acquired := ""
			if parsed, parseErr := time.Parse(time.RFC3339Nano, lock.AcquiredAt); parseErr == nil {
				acquired = parsed.UTC().Format(time.RFC3339Nano)
			}
			locks[name] = map[string]any{"status": status, "pid": lock.PID, "acquired_at": acquired}
		}
		report["locks"] = locks
	}
	body, err := json.Marshal(report)
	if err != nil || len(body) > 64<<10 {
		return []byte(`{"diagnostic_status":"unavailable"}`)
	}
	return body
}

func commandsTimeoutRead(root *os.Root, name string, value any) string {
	info, err := root.Lstat(name)
	if errors.Is(err, os.ErrNotExist) {
		return "absent"
	}
	if err != nil {
		return "unavailable"
	}
	if !info.Mode().IsRegular() || info.Size() > 1<<20 {
		return "refused"
	}
	file, err := root.Open(name)
	if err != nil {
		return "unavailable"
	}
	defer func() { _ = file.Close() }()
	info, err = file.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Size() > 1<<20 {
		return "refused"
	}
	body, err := io.ReadAll(io.LimitReader(file, (1<<20)+1))
	if err != nil || len(body) > 1<<20 {
		return "refused"
	}
	if json.Unmarshal(body, value) != nil {
		return "invalid_json"
	}
	return "read"
}

func commandsTimeoutState(root *os.Root) map[string]any {
	var state struct {
		Installations []struct {
			InstallationID string `json:"installation_id"`
			Clients        map[string]struct {
				ClientID      string `json:"client_id"`
				Activation    string `json:"activation"`
				Verification  string `json:"verification"`
				NativeAttempt string `json:"native_activation_attempt"`
				Pending       *struct {
					AttemptID string `json:"attempt_id"`
					Direction string `json:"direction"`
				} `json:"pending_native_intent"`
			} `json:"clients"`
		} `json:"installations"`
	}
	status := commandsTimeoutRead(root, filepath.Join("installer-state", "state-v2.json"), &state)
	rows := []map[string]any{}
	if status != "read" {
		return map[string]any{"status": status, "bindings": rows, "truncated": false}
	}
	for _, installation := range state.Installations {
		keys := make([]string, 0, len(installation.Clients))
		for key := range installation.Clients {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			if len(rows) == commandsTimeoutEntries {
				return map[string]any{"status": status, "bindings": rows, "truncated": true}
			}
			binding := installation.Clients[key]
			row := map[string]any{"installation_id": commandsTimeoutToken(installation.InstallationID),
				"binding_id": commandsTimeoutToken(key), "client_id": commandsTimeoutToken(binding.ClientID),
				"activation": commandsTimeoutToken(binding.Activation), "verification": commandsTimeoutToken(binding.Verification),
				"native_activation_attempt": commandsTimeoutToken(binding.NativeAttempt), "pending_native_intent": binding.Pending != nil}
			if binding.Pending != nil {
				row["attempt_id"] = commandsTimeoutToken(binding.Pending.AttemptID)
				row["direction"] = commandsTimeoutToken(binding.Pending.Direction)
			}
			rows = append(rows, row)
		}
	}
	return map[string]any{"status": status, "bindings": rows, "truncated": false}
}

func commandsTimeoutJournals(root *os.Root) map[string]any {
	dir := filepath.Join("installer-state", "operations-v2")
	entries, status, truncated := commandsTimeoutDirectory(root, dir)
	rows := []map[string]string{}
	for _, entry := range entries {
		if !entry.Type().IsRegular() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		var journal struct {
			OperationID string `json:"operation_id"`
			Phase       string `json:"phase"`
		}
		read := commandsTimeoutRead(root, filepath.Join(dir, entry.Name()), &journal)
		if read != "read" {
			journal.OperationID, journal.Phase = "", ""
		}
		rows = append(rows, map[string]string{"name": commandsTimeoutToken(entry.Name()), "status": read,
			"operation_id": commandsTimeoutToken(journal.OperationID), "phase": commandsTimeoutToken(journal.Phase)})
	}
	return map[string]any{"status": status, "entries": rows, "truncated": truncated}
}

func commandsTimeoutNames(root *os.Root, name string) map[string]any {
	entries, status, truncated := commandsTimeoutDirectory(root, name)
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		names = append(names, commandsTimeoutToken(entry.Name()))
	}
	return map[string]any{"status": status, "names": names, "truncated": truncated}
}

func commandsTimeoutDirectory(root *os.Root, name string) ([]os.DirEntry, string, bool) {
	info, err := root.Lstat(name)
	if errors.Is(err, os.ErrNotExist) {
		return nil, "absent", false
	}
	if err != nil {
		return nil, "unavailable", false
	}
	if !info.IsDir() {
		return nil, "refused", false
	}
	dir, err := root.Open(name)
	if err != nil {
		return nil, "unavailable", false
	}
	defer func() { _ = dir.Close() }()
	entries, err := dir.ReadDir(commandsTimeoutEntries + 1)
	if err != nil && !errors.Is(err, io.EOF) {
		return nil, "unavailable", false
	}
	truncated := len(entries) > commandsTimeoutEntries
	if truncated {
		entries = entries[:commandsTimeoutEntries]
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
	return entries, "read", truncated
}

func commandsTimeoutToken(value string) string {
	if len(value) > 128 {
		return "<omitted>"
	}
	for _, ch := range value {
		if ch >= 'a' && ch <= 'z' || ch >= 'A' && ch <= 'Z' || ch >= '0' && ch <= '9' || strings.ContainsRune("_-.:@", ch) {
			continue
		}
		return "<omitted>"
	}
	return value
}

func TestCommandsTimeoutSummaryKeepsOnlyBoundedPersistedMetadata(t *testing.T) {
	root := t.TempDir()
	secret := "private credential canary"
	nativeWrite(t, filepath.Join(root, "installer-state", "state-v2.json"), []byte(`{"source_secret":"`+secret+`","installations":[{"installation_id":"owned","clients":{"binding":{"client_id":"codex","activation":"prepared","verification":"installation_verified","native_activation_attempt":"attempt-1","pending_native_intent":{"attempt_id":"attempt-1","direction":"activate","private":"`+secret+`"},"native_profile_root":"`+secret+`"}}}]}`), 0o600)
	nativeWrite(t, filepath.Join(root, "installer-state", "operations-v2", "owned.json"), []byte(`{"operation_id":"owned-operation","phase":"state_committed","private":"`+secret+`"}`), 0o600)
	nativeWrite(t, filepath.Join(root, "installer-state", "mutation.lock"), []byte(`{"pid":42,"acquired_at":"2026-11-16T15:00:00Z","private":"`+secret+`"}`), 0o600)
	for i := range commandsTimeoutEntries + 2 {
		name := strings.Repeat("x", i+1)
		nativeWrite(t, filepath.Join(root, "tmp", name), []byte(secret), 0o600)
	}
	body := commandsTimeoutSummary(root, "replay", 42, 90*time.Second, context.DeadlineExceeded)
	var report struct {
		PID   int `json:"pid"`
		State struct {
			Bindings []map[string]any `json:"bindings"`
		} `json:"state"`
		Scratch struct {
			Names     []string `json:"names"`
			Truncated bool     `json:"truncated"`
		} `json:"scratch"`
	}
	if err := json.Unmarshal(body, &report); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(body), secret) || len(body) > 64<<10 || report.PID != 42 || len(report.State.Bindings) != 1 || report.State.Bindings[0]["attempt_id"] != "attempt-1" || !strings.Contains(string(body), `"phase":"state_committed"`) || !report.Scratch.Truncated || len(report.Scratch.Names) != commandsTimeoutEntries {
		t.Fatalf("timeout metadata missing or over-disclosed: %s", body)
	}
	commandsRecordTimeout(t, root, "replay", 42, 90*time.Second, context.DeadlineExceeded)
	retained, err := os.ReadFile(filepath.Join(root, "replay-installer-timeout.json"))
	if err != nil || !json.Valid(retained) || strings.Contains(string(retained), secret) {
		t.Fatalf("bounded diagnostic was not retained safely: %v", err)
	}
}

func TestCommandsTimeoutSummaryRefusesOutsideAndOversizedState(t *testing.T) {
	for _, mode := range []string{"outside", "oversized", "invalid"} {
		t.Run(mode, func(t *testing.T) {
			root := t.TempDir()
			path := filepath.Join(root, "installer-state", "state-v2.json")
			if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
				t.Fatal(err)
			}
			switch mode {
			case "outside":
				foreign := filepath.Join(t.TempDir(), "outside.json")
				nativeWrite(t, foreign, []byte(`{"installations":[{"installation_id":"FOREIGN-CANARY"}]}`), 0o600)
				if err := os.Symlink(foreign, path); err != nil {
					t.Skipf("symlink prerequisite unavailable: %v", err)
				}
			case "oversized":
				nativeWrite(t, path, []byte(strings.Repeat("x", (1<<20)+1)), 0o600)
			default:
				nativeWrite(t, path, []byte(`{"installations":`), 0o600)
			}
			body := commandsTimeoutSummary(root, "replay", 42, 90*time.Second, context.DeadlineExceeded)
			var report struct {
				State struct {
					Status   string `json:"status"`
					Bindings []any  `json:"bindings"`
				} `json:"state"`
			}
			if json.Unmarshal(body, &report) != nil || report.State.Status == "read" || len(report.State.Bindings) != 0 || strings.Contains(string(body), "FOREIGN-CANARY") {
				t.Fatalf("unsafe state was read: %s", body)
			}
		})
	}
}
