package pluginkitairepo_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"
)

const commandsTimeoutEntries = 32

// This is a post-return snapshot of persisted fixture metadata. It neither
// observes a live child nor changes cancellation, cleanup or the original fatal.
func commandsRecordTimeout(t *testing.T, root, label string, cmd *exec.Cmd, elapsed time.Duration, cause error) {
	t.Helper()
	body := commandsTimeoutSummary(root, label, cmd, elapsed, cause)
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

func commandsTimeoutSummary(root, label string, cmd *exec.Cmd, elapsed time.Duration, cause error) []byte {
	reason := "canceled"
	if errors.Is(cause, context.DeadlineExceeded) {
		reason = "deadline_exceeded"
	}
	pid := 0
	if cmd.Process != nil {
		pid = cmd.Process.Pid
	}
	report := map[string]any{"label": commandsTimeoutToken(label), "pid": pid,
		"elapsed_ms": elapsed.Milliseconds(), "reason": reason,
		"scope": "persisted fixture metadata only; live child and internal phase not inferred"}
	base, err := os.OpenRoot(root)
	if err != nil {
		report["root_status"] = "unavailable"
	} else {
		defer func() { _ = base.Close() }()
		report["user"] = commandsTimeoutMetadata(base, "installer-state")
		report["project"] = commandsTimeoutProject(base, root, cmd)
		report["scratch"] = commandsTimeoutNames(base, "tmp")
	}
	body, err := json.Marshal(report)
	if err != nil || len(body) > 64<<10 {
		return []byte(`{"diagnostic_status":"unavailable"}`)
	}
	return body
}

func commandsTimeoutMetadata(root *os.Root, stateDir string) map[string]any {
	report := map[string]any{
		"state":    commandsTimeoutState(root, stateDir),
		"journals": commandsTimeoutJournals(root, stateDir),
		"logs":     commandsTimeoutNames(root, filepath.Join(stateDir, "logs")),
	}
	locks := map[string]any{}
	for _, name := range []string{"environment.lock", "mutation.lock"} {
		var lock struct {
			PID        int    `json:"pid"`
			AcquiredAt string `json:"acquired_at"`
		}
		status := commandsTimeoutRead(root, filepath.Join(stateDir, name), &lock)
		if status != "read" {
			lock.PID, lock.AcquiredAt = 0, ""
		}
		acquired := ""
		if parsed, err := time.Parse(time.RFC3339Nano, lock.AcquiredAt); err == nil {
			acquired = parsed.UTC().Format(time.RFC3339Nano)
		}
		locks[name] = map[string]any{"status": status, "pid": lock.PID, "acquired_at": acquired}
	}
	report["locks"] = locks
	return report
}

func commandsTimeoutProject(root *os.Root, fixture string, cmd *exec.Cmd) map[string]any {
	scope, project := commandsTimeoutSelection(cmd.Args)
	if scope != "project" {
		return map[string]any{"status": "not_selected"}
	}
	if project == "" {
		project = cmd.Dir
	} else if !filepath.IsAbs(project) {
		project = filepath.Join(cmd.Dir, project)
	}
	owned, ownedErr := filepath.EvalSymlinks(fixture)
	selected, selectedErr := filepath.EvalSymlinks(project)
	if ownedErr != nil || selectedErr != nil {
		return map[string]any{"scope": "project", "status": "unavailable"}
	}
	relative, err := filepath.Rel(owned, selected)
	if err != nil || !filepath.IsLocal(relative) {
		return map[string]any{"scope": "project", "status": "outside_fixture"}
	}
	base, err := root.OpenRoot(relative)
	if err != nil {
		return map[string]any{"scope": "project", "status": "unavailable"}
	}
	defer func() { _ = base.Close() }()
	report := commandsTimeoutMetadata(base, ".uap")
	report["scope"], report["status"] = "project", "read"
	files := map[string]any{}
	for _, name := range []string{"uap.toml", "uap.lock"} {
		files[name] = commandsTimeoutFileMetadata(base, name)
	}
	report["environment_files"] = files
	return report
}

func commandsTimeoutSelection(args []string) (string, string) {
	scope, project := "user", ""
	for i := 1; i < len(args); i++ {
		flag, value, assigned := strings.Cut(args[i], "=")
		if flag == "--" {
			break
		}
		if flag != "--scope" && flag != "--project-root" {
			continue
		}
		if !assigned {
			i++
			if i == len(args) {
				return "invalid", ""
			}
			value = args[i]
		}
		if flag == "--scope" {
			scope = value
		} else {
			project = value
		}
	}
	return scope, project
}

func commandsTimeoutFileMetadata(root *os.Root, name string) map[string]any {
	info, err := root.Lstat(name)
	if errors.Is(err, os.ErrNotExist) {
		return map[string]any{"status": "absent"}
	}
	if err != nil {
		return map[string]any{"status": "unavailable"}
	}
	if !info.Mode().IsRegular() {
		return map[string]any{"status": "refused"}
	}
	return map[string]any{"status": "present", "bytes": info.Size()}
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

func commandsTimeoutState(root *os.Root, stateDir string) map[string]any {
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
	status := commandsTimeoutRead(root, filepath.Join(stateDir, "state-v2.json"), &state)
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

func commandsTimeoutJournals(root *os.Root, stateDir string) map[string]any {
	dir := filepath.Join(stateDir, "operations-v2")
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
	body := commandsTimeoutSummary(root, "replay", &exec.Cmd{Process: &os.Process{Pid: 42}}, 90*time.Second, context.DeadlineExceeded)
	var report struct {
		PID  int `json:"pid"`
		User struct {
			State struct {
				Bindings []map[string]any `json:"bindings"`
			} `json:"state"`
		} `json:"user"`
		Scratch struct {
			Names     []string `json:"names"`
			Truncated bool     `json:"truncated"`
		} `json:"scratch"`
	}
	if err := json.Unmarshal(body, &report); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(body), secret) || len(body) > 64<<10 || report.PID != 42 || len(report.User.State.Bindings) != 1 || report.User.State.Bindings[0]["attempt_id"] != "attempt-1" || !strings.Contains(string(body), `"phase":"state_committed"`) || !report.Scratch.Truncated || len(report.Scratch.Names) != commandsTimeoutEntries {
		t.Fatalf("timeout metadata missing or over-disclosed: %s", body)
	}
	commandsRecordTimeout(t, root, "replay", &exec.Cmd{Process: &os.Process{Pid: 42}}, 90*time.Second, context.DeadlineExceeded)
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
			body := commandsTimeoutSummary(root, "replay", &exec.Cmd{Process: &os.Process{Pid: 42}}, 90*time.Second, context.DeadlineExceeded)
			var report struct {
				User struct {
					State struct {
						Status   string `json:"status"`
						Bindings []any  `json:"bindings"`
					} `json:"state"`
				} `json:"user"`
			}
			if json.Unmarshal(body, &report) != nil || report.User.State.Status == "read" || len(report.User.State.Bindings) != 0 || strings.Contains(string(body), "FOREIGN-CANARY") {
				t.Fatalf("unsafe state was read: %s", body)
			}
		})
	}
}

func TestCommandsTimeoutSummarySelectsOwnedProjectMetadata(t *testing.T) {
	root := t.TempDir()
	project := filepath.Join(root, "project")
	secret := "private project credential canary"
	nativeWrite(t, filepath.Join(project, ".uap", "state-v2.json"), []byte(`{"private":"`+secret+`","installations":[{"installation_id":"project-owned","clients":{"binding":{"client_id":"codex","activation":"active"}}}]}`), 0o600)
	nativeWrite(t, filepath.Join(project, ".uap", "operations-v2", "owned.json"), []byte(`{"operation_id":"project-operation","phase":"state_committed","private":"`+secret+`"}`), 0o600)
	nativeWrite(t, filepath.Join(project, ".uap", "mutation.lock"), []byte(`{"pid":73,"private":"`+secret+`"}`), 0o600)
	for _, name := range []string{"uap.toml", "uap.lock"} {
		nativeWrite(t, filepath.Join(project, name), []byte(secret), 0o600)
	}
	for _, args := range [][]string{
		{"installer", "skills", "add", "source", "--scope", "project", "--project-root", project},
		{"installer", "--scope=project", "--project-root=."},
		{"installer", "--scope", "project"},
	} {
		cmd := &exec.Cmd{Dir: project, Args: args}
		body := commandsTimeoutSummary(root, "project-replay", cmd, 90*time.Second, context.DeadlineExceeded)
		var report struct {
			User struct {
				State struct{ Status string } `json:"state"`
			} `json:"user"`
			Project struct {
				Scope, Status string
				State         struct {
					Status   string
					Bindings []map[string]any
				}
				Locks map[string]struct{ PID int }
				Files map[string]struct {
					Status string
					Bytes  int64
				} `json:"environment_files"`
			} `json:"project"`
		}
		if err := json.Unmarshal(body, &report); err != nil {
			t.Fatal(err)
		}
		p := report.Project
		if report.User.State.Status != "absent" || p.Scope != "project" || p.Status != "read" || p.State.Status != "read" || len(p.State.Bindings) != 1 || p.State.Bindings[0]["installation_id"] != "project-owned" || p.Locks["mutation.lock"].PID != 73 {
			t.Fatalf("selected project metadata missing: %s", body)
		}
		if strings.Contains(string(body), secret) || strings.Contains(string(body), project) || !strings.Contains(string(body), `"operation_id":"project-operation"`) || p.Files["uap.toml"].Status != "present" || p.Files["uap.lock"].Bytes != int64(len(secret)) {
			t.Fatalf("project metadata missing or over-disclosed: %s", body)
		}
	}
}

func TestCommandsTimeoutProjectRequiresSelectionAndOwnedContainment(t *testing.T) {
	root, outside := t.TempDir(), t.TempDir()
	foreign := `{"installations":[{"installation_id":"FOREIGN-CANARY","clients":{"foreign":{"client_id":"codex"}}}]}`
	nativeWrite(t, filepath.Join(outside, ".uap", "state-v2.json"), []byte(foreign), 0o600)
	base, err := os.OpenRoot(root)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = base.Close() }()
	for _, tc := range []struct {
		args   []string
		status string
	}{
		{[]string{"installer", "--project-root", outside}, "not_selected"},
		{[]string{"installer", "--scope", "project", "--scope", "user", "--project-root", outside}, "not_selected"},
		{[]string{"installer", "--", "--scope", "project", "--project-root", outside}, "not_selected"},
		{[]string{"installer", "--scope", "project", "--project-root", outside}, "outside_fixture"},
		{[]string{"installer", "--scope=project", "--project-root=../" + filepath.Base(outside)}, "outside_fixture"},
	} {
		report := commandsTimeoutProject(base, root, &exec.Cmd{Dir: root, Args: tc.args})
		body, err := json.Marshal(report)
		if err != nil || report["status"] != tc.status || strings.Contains(string(body), "FOREIGN-CANARY") || strings.Contains(string(body), outside) {
			t.Fatalf("unselected or outside metadata was read: %s %v", body, err)
		}
	}
	project := filepath.Join(root, "project")
	if err := os.Mkdir(project, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(outside, ".uap"), filepath.Join(project, ".uap")); err != nil {
		t.Skipf("symlink prerequisite unavailable: %v", err)
	}
	report := commandsTimeoutProject(base, root, &exec.Cmd{Dir: project, Args: []string{"installer", "--scope", "project"}})
	body, err := json.Marshal(report)
	state, ok := report["state"].(map[string]any)
	if err != nil || !ok || state["status"] == "read" || strings.Contains(string(body), "FOREIGN-CANARY") {
		t.Fatalf("outside project state was read: %s %v", body, err)
	}
}
