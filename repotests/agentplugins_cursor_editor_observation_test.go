//go:build linux

package pluginkitairepo_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/process"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

const cursorEditorPin = "c8003e009f96a6e4fd7185f7016c66d7e36e03d8fa2e2394197511907bcd8226"

// The exact archive above contains 16,137 entries under usr/share/cursor,
// including that root: 13,212 regular files and 2,925 directories. Its full
// 1,037,799,007 regular-file bytes remain below the unchanged 1 GiB tree cap.
// Only the archive-verified runtime receives this measured entry capacity.
const cursorEditorRuntimeEntries = 16137

func cursorEditorTreeEntryAllowed(label string, admitted int) bool {
	limit := 10000
	if label == "runtime" {
		limit = cursorEditorRuntimeEntries
	}
	return admitted < limit
}

// Only internal runtime aliases are admitted; no alias is ever traversed by
// the walk. Regular files use the existing held no-follow snapshot reader.
func cursorEditorTree(ctx context.Context, root, label string) (map[string]acpMetadataFile, error) {
	facts := map[string]acpMetadataFile{}
	var total int64
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil || ctx.Err() != nil || !cursorEditorTreeEntryAllowed(label, len(facts)) {
			return fmt.Errorf("editor tree boundary")
		}
		info, err := os.Lstat(path)
		if err != nil {
			return err
		}
		var fact acpMetadataFile
		if info.Mode()&os.ModeSymlink != 0 {
			fact, err = cursorEditorAlias(root, path, info)
		} else {
			fact, err = acpMetadataFileFact(path)
		}
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		facts[filepath.ToSlash(relative)] = fact
		if entry.Type().IsRegular() {
			total += fact.Bytes
		}
		if total > 1<<30 {
			return fmt.Errorf("editor tree byte bound")
		}
		return nil
	})
	return facts, err
}

func cursorEditorAlias(root, path string, before os.FileInfo) (acpMetadataFile, error) {
	target, err := os.Readlink(path)
	resolved, resolveErr := filepath.EvalSymlinks(path)
	relative, relativeErr := filepath.Rel(root, resolved)
	stat, ok := before.Sys().(*syscall.Stat_t)
	if err != nil || resolveErr != nil || relativeErr != nil || !ok || stat.Nlink != 1 ||
		(stat.Uid != uint32(os.Getuid()) && stat.Uid != 0) || filepath.IsAbs(relative) ||
		relative == ".." || strings.HasPrefix(relative, ".."+string(os.PathSeparator)) || len(target) > 4096 {
		return acpMetadataFile{}, fmt.Errorf("editor runtime alias refused")
	}
	after, err := os.Lstat(path)
	if err != nil || !os.SameFile(before, after) || before.Mode() != after.Mode() ||
		!before.ModTime().Equal(after.ModTime()) {
		return acpMetadataFile{}, fmt.Errorf("editor runtime alias changed")
	}
	digest := sha256.Sum256([]byte(target))
	return acpMetadataFile{Mode: uint32(before.Mode()), Bytes: before.Size(), SHA: hex.EncodeToString(digest[:]),
		Identity: []any{stat.Dev, stat.Ino, stat.Nlink, stat.Uid}, info: before}, nil
}

func cursorEditorCapture(ctx context.Context, argv, env []string, cwd string, input []byte) ([]byte, error, map[string]any) {
	var body []byte
	facts := map[string]any{"exchange_phase": "not-started", "stdin_bytes": 0}
	err := (process.OS{}).RunDuplexToCompletion(ctx, ports.Command{Argv: argv, Env: env, Dir: cwd},
		func(writer io.Writer, reader io.Reader) error {
			facts["exchange_phase"] = "stdin-write"
			if len(input) > 0 {
				n, err := writer.Write(input)
				facts["stdin_bytes"] = n
				if err != nil {
					return err
				}
			}
			facts["exchange_phase"] = "stdin-close"
			closer, ok := writer.(io.Closer)
			if !ok {
				return fmt.Errorf("editor input closure unavailable")
			}
			if err := closer.Close(); err != nil {
				return err
			}
			facts["exchange_phase"] = "stdout-read"
			var err error
			body, err = io.ReadAll(io.LimitReader(reader, (1<<20)+1))
			if err != nil || len(body) > 1<<20 {
				return fmt.Errorf("editor output read or bound")
			}
			facts["exchange_phase"] = "completed"
			return nil
		})
	for key, value := range cursorEditorCompletionFacts(ctx, err) {
		facts[key] = value
	}
	return body, err, facts
}

func cursorEditorRefusalCause(err error) string {
	if err == nil {
		return "unclassified"
	}
	switch err.Error() {
	case "editor tree boundary":
		return "tree-boundary"
	case "editor tree byte bound":
		return "aggregate-bytes"
	case "metadata custody type/owner":
		return "type-owner"
	case "metadata custody hardlink":
		return "hardlink"
	case "editor runtime alias refused":
		return "alias-refused"
	case "editor runtime alias changed":
		return "alias-changed"
	case "snapshot pre-read type/size refused":
		return "file-type-size"
	case "snapshot opened identity differs", "snapshot bytes/held identity changed", "snapshot path identity changed":
		return "held-identity"
	}
	for _, prefix := range []string{"snapshot opened identity: ", "snapshot held identity: ", "snapshot path identity: "} {
		if strings.HasPrefix(err.Error(), prefix) {
			return "held-read"
		}
	}
	return "unclassified"
}

func cursorEditorRefusalFacts(label string, facts map[string]acpMetadataFile, err error) string {
	switch label {
	case "runtime", "project-skills", "user-plugin", "playwright", "playwright-core":
	default:
		label = "unknown"
	}
	var total int64
	for _, fact := range facts {
		if os.FileMode(fact.Mode).IsRegular() {
			total += fact.Bytes
		}
	}
	// Count only facts already captured by the unchanged walk. Its final
	// individually valid file remains counted if it trips the aggregate bound.
	return fmt.Sprintf("editor frozen tree refusal root=%s cause=%s admitted_entries=%d admitted_regular_bytes=%d",
		label, cursorEditorRefusalCause(err), len(facts), total)
}

func cursorEditorFrozen(ctx context.Context, t *testing.T, roots map[string]string) map[string]map[string]acpMetadataFile {
	t.Helper()
	result := map[string]map[string]acpMetadataFile{}
	for label, root := range roots {
		resolved, err := filepath.EvalSymlinks(root)
		if err != nil || resolved != root || !filepath.IsAbs(root) {
			t.Fatal("editor frozen root authority")
		}
		facts, err := cursorEditorTree(ctx, root, label)
		if err != nil {
			t.Log(cursorEditorRefusalFacts(label, facts, err))
			t.Fatalf("editor frozen tree refused: %s", label)
		}
		result[label] = facts
	}
	return result
}

func TestCursorEditorPinnedRuntimeEntryCapacity(t *testing.T) {
	root := t.TempDir()
	for index := range cursorEditorRuntimeEntries - 1 {
		if err := os.WriteFile(filepath.Join(root, fmt.Sprintf("entry-%05d", index)), nil, 0600); err != nil {
			t.Fatal(err)
		}
	}
	facts, err := cursorEditorTree(t.Context(), root, "runtime")
	if err != nil || len(facts) != cursorEditorRuntimeEntries {
		t.Fatalf("pinned runtime capacity refused: entries=%d error=%v", len(facts), err)
	}
	facts, err = cursorEditorTree(t.Context(), root, "project-skills")
	if err == nil || len(facts) != 10000 || cursorEditorRefusalCause(err) != "tree-boundary" {
		t.Fatalf("ordinary tree entry cap changed: entries=%d error=%v", len(facts), err)
	}
	if err := os.WriteFile(filepath.Join(root, "extra"), nil, 0600); err != nil {
		t.Fatal(err)
	}
	facts, err = cursorEditorTree(t.Context(), root, "runtime")
	if err == nil || len(facts) != cursorEditorRuntimeEntries || cursorEditorRefusalCause(err) != "tree-boundary" {
		t.Fatalf("extra runtime entry admitted: entries=%d error=%v", len(facts), err)
	}
	for _, label := range []string{"", "project-skills", "user-plugin", "playwright", "playwright-core", "unknown"} {
		if !cursorEditorTreeEntryAllowed(label, 9999) || cursorEditorTreeEntryAllowed(label, 10000) {
			t.Fatalf("ordinary tree entry cap changed: %q", label)
		}
	}
}

func TestCursorEditorFreezeRefusalFacts(t *testing.T) {
	for _, scenario := range []struct {
		name, cause string
		entries     int
		bytes       int64
	}{
		{"ordinary", "", 2, 5},
		{"hardlink", "hardlink", 1, 0},
		{"external-alias", "alias-refused", 2, 5},
		{"special", "type-owner", 2, 5},
		{"large", "file-type-size", 2, 5},
		{"canceled", "tree-boundary", 0, 0},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			root := t.TempDir()
			safe, private := filepath.Join(root, "00-safe"), filepath.Join(root, "10-PRIVATE-CANARY")
			if err := os.WriteFile(safe, []byte("owned"), 0600); err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			switch scenario.name {
			case "hardlink":
				if err := os.Link(safe, private); err != nil {
					t.Fatal(err)
				}
			case "external-alias":
				outside := filepath.Join(t.TempDir(), "PRIVATE-CANARY")
				if err := os.WriteFile(outside, []byte("foreign owned control"), 0600); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(outside, private); err != nil {
					t.Fatal(err)
				}
			case "special":
				if err := syscall.Mkfifo(private, 0600); err != nil {
					t.Fatal(err)
				}
			case "large":
				file, err := os.OpenFile(private, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
				if err != nil {
					t.Fatal(err)
				}
				truncateErr, closeErr := file.Truncate((512<<20)+1), file.Close()
				if truncateErr != nil || closeErr != nil {
					t.Fatal(truncateErr, closeErr)
				}
			case "canceled":
				cancel()
			}
			facts, err := cursorEditorTree(ctx, root, "runtime")
			if scenario.cause == "" {
				if err != nil || len(facts) != scenario.entries || facts["00-safe"].Bytes != scenario.bytes {
					t.Fatal("ordinary custody control failed", err)
				}
				return
			}
			want := fmt.Sprintf("editor frozen tree refusal root=runtime cause=%s admitted_entries=%d admitted_regular_bytes=%d", scenario.cause, scenario.entries, scenario.bytes)
			if err == nil || cursorEditorRefusalFacts("runtime", facts, err) != want {
				t.Fatal("actual refusal did not retain exact safe facts")
			}
		})
	}
}

func TestCursorEditorFreezeRefusalPrivacyAndFixedCauses(t *testing.T) {
	for _, scenario := range []struct{ message, cause string }{
		{"editor tree byte bound", "aggregate-bytes"},
		{"editor runtime alias changed", "alias-changed"},
		{"snapshot opened identity differs", "held-identity"},
		{"snapshot bytes/held identity changed", "held-identity"},
		{"snapshot path identity changed", "held-identity"},
		{"snapshot opened identity: PRIVATE-CANARY", "held-read"},
		{"snapshot held identity: PRIVATE-CANARY", "held-read"},
		{"snapshot path identity: PRIVATE-CANARY", "held-read"},
		{"/home/PRIVATE-CANARY token=PRIVATE-CANARY\nraw output", "unclassified"},
	} {
		facts := map[string]acpMetadataFile{
			"PRIVATE-CANARY": {Mode: 0600, Bytes: 5, SHA: "PRIVATE-CANARY"},
			"private-dir":    {Mode: uint32(os.ModeDir | 0700), Bytes: 4096},
			"private-alias":  {Mode: uint32(os.ModeSymlink | 0777), Bytes: 100},
		}
		got := cursorEditorRefusalFacts("/home/PRIVATE-CANARY", facts, fmt.Errorf("%s", scenario.message))
		want := fmt.Sprintf("editor frozen tree refusal root=unknown cause=%s admitted_entries=3 admitted_regular_bytes=5", scenario.cause)
		if got != want || strings.Contains(got, "PRIVATE-CANARY") {
			t.Fatal("refusal leaked private contents or lost fixed classification")
		}
	}
}

func cursorEditorFiles(t *testing.T, paths map[string]string) map[string]acpMetadataFile {
	t.Helper()
	result := map[string]acpMetadataFile{}
	for label, path := range paths {
		resolved, err := filepath.EvalSymlinks(path)
		if err != nil || resolved != path || !filepath.IsAbs(path) {
			t.Fatal("editor input path authority")
		}
		fact, err := acpMetadataFileFact(path)
		if err != nil || !fact.info.Mode().IsRegular() {
			t.Fatalf("editor input refused: %s", label)
		}
		result[label] = fact
	}
	return result
}

func cursorEditorSummary(t *testing.T, facts map[string]acpMetadataFile) map[string]any {
	t.Helper()
	body, err := json.Marshal(facts)
	if err != nil {
		t.Fatal("editor custody summary encoding")
	}
	digest := sha256.Sum256(body)
	return map[string]any{"entries": len(facts), "sha256": hex.EncodeToString(digest[:])}
}

func cursorEditorSave(t *testing.T, directory string, report map[string]any) {
	t.Helper()
	body, err := json.MarshalIndent(report, "", "  ")
	if err != nil || len(body) > 2<<20 {
		t.Error("editor observation report bound")
		return
	}
	nativeWrite(t, filepath.Join(directory, "metadata.json"), append(body, '\n'), 0600)
}

func cursorEditorInput(t *testing.T, root, project, output, source, electron string, help []byte) map[string]any {
	t.Helper()
	home := filepath.Join(root, "home")
	plugin := filepath.Join(home, ".cursor", "plugins", "local", "uap-editor-inert-plugin")
	pluginSkill := filepath.Join(plugin, "skills", "uap-editor-plugin-skill", "SKILL.md")
	nativeWrite(t, filepath.Join(plugin, "plugin.json"), []byte(`{"$schema":"https://agent-plugins.org/schemas/1.0.0/plugin.schema.json","name":"uap-editor-inert-plugin","version":"1.0.0","description":"Inert UI inventory fixture"}`), 0600)
	nativeWrite(t, pluginSkill, []byte("---\nname: uap-editor-plugin-skill\ndescription: Inert UI inventory fixture\n---\nDo not execute this Skill.\n"), 0600)
	for _, name := range []string{"userdata", "extensions"} {
		if err := os.Mkdir(filepath.Join(root, name), 0700); err != nil {
			t.Fatal("editor profile creation")
		}
	}
	return map[string]any{"root": root, "project": project, "userData": filepath.Join(root, "userdata"),
		"extensions": filepath.Join(root, "extensions"), "output": output, "electron": electron,
		"playwright": filepath.Join(source, "ui", "skills-picker", "node_modules", "playwright"),
		"help":       string(help), "projectSkill": acpMetadataMarker, "pluginName": "uap-editor-inert-plugin",
		"pluginSkill": "uap-editor-plugin-skill", "projectSkillPath": filepath.Join(project, ".cursor", "skills", acpMetadataMarker, "SKILL.md"),
		"pluginSkillPath": pluginSkill}
}

func TestAgentpluginsCursorEditorObservation(t *testing.T) {
	if os.Getenv("AGENTPLUGINS_CURSOR_EDITOR_OBSERVE") != "1" {
		t.Skip("explicit disposable Cursor Editor UI observation only")
	}
	nativeRequireDisposable(t)
	if os.Getenv("GITHUB_ACTIONS") != "true" || os.Getenv("RUNNER_ENVIRONMENT") != "github-hosted" || os.Getenv("DISPLAY") == "" {
		t.Fatal("ordinary hosted display required")
	}
	temp, err := filepath.EvalSymlinks(os.Getenv("RUNNER_TEMP"))
	if err != nil || !filepath.IsAbs(temp) {
		t.Fatal("ordinary hosted scratch required")
	}
	output := filepath.Join(temp, "cursor-editor-observation")
	if err := os.Mkdir(output, 0700); err != nil {
		t.Fatal("fresh editor output required")
	}
	report := map[string]any{"status": "unqualified", "skills_ready": false, "model_runtime_qualified": false, "authentication_qualified": false, "source_attribution_qualified": false}
	defer cursorEditorSave(t, output, report)
	source, err := nativeSourceIdentity(os.Getenv("AGENTPLUGINS_CURSOR_EDITOR_COMMIT"), os.Getenv("AGENTPLUGINS_CURSOR_EDITOR_TREE"), "")
	if err != nil {
		t.Fatal("editor source identity required")
	}
	report["source"] = source
	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Minute)
	defer cancel()
	cursorEditorObserve(ctx, t, temp, output, report)
}

func cursorEditorObserve(ctx context.Context, t *testing.T, temp, output string, report map[string]any) {
	t.Helper()
	runtimeRoot := filepath.Join(temp, "vendor-root", "usr", "share", "cursor")
	source := os.Getenv("AGENTPLUGINS_CURSOR_EDITOR_SOURCE")
	node := os.Getenv("AGENTPLUGINS_CURSOR_EDITOR_NODE")
	paths := map[string]string{"archive": filepath.Join(temp, "cursor.deb"), "launcher": filepath.Join(runtimeRoot, "bin", "cursor"),
		"version":  filepath.Join(temp, "vendor-proof", "version.txt"),
		"electron": filepath.Join(runtimeRoot, "cursor"), "driver": filepath.Join(source, "scripts", "cursor_editor_observation.cjs"), "node": node}
	filesBefore := cursorEditorFiles(t, paths)
	if filesBefore["archive"].SHA != cursorEditorPin {
		t.Fatal("Cursor Editor exact official archive differs")
	}
	runtimeBefore := cursorEditorFrozen(ctx, t, map[string]string{"runtime": runtimeRoot})["runtime"]
	report["runtime_before_help"] = cursorEditorSummary(t, runtimeBefore)
	root, project, env, _ := acpMetadataProfile(t, "cursor")
	for _, key := range []string{"DISPLAY", "XAUTHORITY"} {
		if value := os.Getenv(key); value != "" {
			env = append(env, key+"="+value)
		}
	}
	cursorEditorHelpAndUI(ctx, t, root, project, output, source, paths, env, filesBefore, runtimeBefore, report)
}

func cursorEditorHelpAndUI(ctx context.Context, t *testing.T, root, project, output, source string, paths map[string]string, env []string, filesBefore, runtimeBefore map[string]acpMetadataFile, report map[string]any) {
	t.Helper()
	report["stage"] = "native-help"
	probe, stop := context.WithTimeout(ctx, 30*time.Second)
	help, helpErr, _ := cursorEditorCapture(probe, []string{paths["launcher"], "--help"}, env, project, nil)
	stop()
	nativeWrite(t, filepath.Join(output, "help.txt"), help, 0600)
	if helpErr != nil || len(help) > 65536 {
		t.Fatal("exact editor help did not complete naturally")
	}
	info, err := os.Lstat(paths["version"])
	if err != nil {
		t.Fatal("editor measured version absent")
	}
	body, err := readProjectNativeSnapshot(paths["version"], info, 65536)
	if err != nil || strings.Split(strings.TrimSpace(string(body)), "\n")[0] != "3.30.14" {
		t.Fatal("editor measured version differs")
	}
	runtimeAfterHelp := cursorEditorFrozen(ctx, t, map[string]string{"runtime": filepath.Dir(paths["electron"])})["runtime"]
	if !acpMetadataSame(runtimeBefore, runtimeAfterHelp) || !acpMetadataSame(filesBefore, cursorEditorFiles(t, paths)) {
		t.Fatal("editor help changed frozen acquisition")
	}
	input := cursorEditorInput(t, root, project, output, source, paths["electron"], help)
	roots := map[string]string{"runtime": filepath.Dir(paths["electron"]), "project-skills": filepath.Join(project, ".cursor"),
		"user-plugin":     filepath.Join(root, "home", ".cursor", "plugins", "local"),
		"playwright":      filepath.Join(source, "ui", "skills-picker", "node_modules", "playwright"),
		"playwright-core": filepath.Join(source, "ui", "skills-picker", "node_modules", "playwright-core")}
	before := cursorEditorFrozen(ctx, t, roots)
	summariesBefore := map[string]any{}
	for label, facts := range before {
		summariesBefore[label] = cursorEditorSummary(t, facts)
	}
	report["inputs_before"], report["trees_before"] = filesBefore, summariesBefore
	encoded, err := json.Marshal(input)
	if err != nil || len(encoded) > 65536 {
		t.Fatal("editor input encoding or bound")
	}
	report["stage"] = "editor-ui"
	observed, runErr, captureFacts := cursorEditorCapture(ctx, []string{paths["node"], paths["driver"]}, env, project, encoded)
	report["driver_completion"] = captureFacts
	stdoutDigest := sha256.Sum256(observed)
	report["driver_stdout_bytes"], report["driver_stdout_sha256"] = len(observed), hex.EncodeToString(stdoutDigest[:])
	var result map[string]any
	decodeErr := json.Unmarshal(observed, &result)
	after, filesAfter := cursorEditorFrozen(ctx, t, roots), cursorEditorFiles(t, paths)
	summariesAfter := map[string]any{}
	for label, facts := range after {
		summariesAfter[label] = cursorEditorSummary(t, facts)
	}
	report["inputs_after"], report["trees_after"], report["ui"] = filesAfter, summariesAfter, result
	unchanged := acpMetadataSame(filesBefore, filesAfter)
	for label, previous := range before {
		unchanged = unchanged && acpMetadataSame(previous, after[label])
	}
	report["custody_unchanged"], report["completion_class"] = unchanged, acpMetadataCompletionClass(runErr)
	if runErr != nil || decodeErr != nil || !unchanged || result["status"] != "observed" || result["skills_ready"] != false {
		t.Fatal("Cursor Editor UI observation remains unqualified; inspect bounded metadata and screenshots")
	}
	report["status"] = "component-names-observed"
}

// Error identities are diagnostic facts only; an observed exit never overrides
// supervisor failure or changes the natural-completion acceptance predicate.
func cursorEditorCompletionFacts(ctx context.Context, err error) map[string]any {
	facts := map[string]any{"error_present": err != nil,
		"supervisor_failure":      process.OrdinarySupervisorFailureClass(err),
		"forced_cleanup":          errors.Is(err, process.ErrOrdinaryForcedCleanup),
		"containment_unsupported": errors.Is(err, process.ErrOrdinaryContainmentUnsupported)}
	if ctx.Err() != nil {
		facts["context"] = acpMetadataCompletionClass(ctx.Err())
	}
	var exit *exec.ExitError
	if errors.As(err, &exit) && exit.ProcessState != nil {
		facts["observed_exit_code"] = exit.ExitCode()
		if status, ok := exit.Sys().(syscall.WaitStatus); ok && status.Signaled() {
			facts["observed_signal"] = int(status.Signal())
		}
	}
	var errno syscall.Errno
	if errors.As(err, &errno) {
		facts["observed_errno"] = uint64(errno)
	}
	return facts
}

func TestCursorEditorCompletionFactsNeverPublishNativeContents(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := errors.Join(fmt.Errorf("PRIVATE-CANARY %w", syscall.EACCES), process.ErrOrdinaryForcedCleanup)
	facts := cursorEditorCompletionFacts(ctx, err)
	encoded, marshalErr := json.Marshal(facts)
	if marshalErr != nil || strings.Contains(string(encoded), "PRIVATE-CANARY") ||
		facts["context"] != "canceled" || facts["observed_errno"] != uint64(syscall.EACCES) ||
		facts["error_present"] != true || facts["forced_cleanup"] != true || facts["supervisor_failure"] != "" {
		t.Fatal("driver completion diagnostics leaked contents or lost error identity")
	}
	if got := cursorEditorCompletionFacts(context.Background(), nil); got["error_present"] != false || got["forced_cleanup"] != false {
		t.Fatal("successful completion diagnostics invented errors")
	}
}
