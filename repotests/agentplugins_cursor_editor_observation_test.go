//go:build linux

package pluginkitairepo_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/process"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

const cursorEditorPin = "c8003e009f96a6e4fd7185f7016c66d7e36e03d8fa2e2394197511907bcd8226"

// Only internal runtime aliases are admitted; no alias is ever traversed by
// the walk. Regular files use the existing held no-follow snapshot reader.
func cursorEditorTree(ctx context.Context, root string) (map[string]acpMetadataFile, error) {
	facts := map[string]acpMetadataFile{}
	var total int64
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil || ctx.Err() != nil || len(facts) >= 10000 {
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

func cursorEditorCapture(ctx context.Context, argv, env []string, cwd string, input []byte) ([]byte, error) {
	var body []byte
	err := (process.OS{}).RunDuplexToCompletion(ctx, ports.Command{Argv: argv, Env: env, Dir: cwd},
		func(writer io.Writer, reader io.Reader) error {
			if len(input) > 0 {
				if _, err := writer.Write(input); err != nil {
					return err
				}
			}
			closer, ok := writer.(io.Closer)
			if !ok {
				return fmt.Errorf("editor input closure unavailable")
			}
			if err := closer.Close(); err != nil {
				return err
			}
			var err error
			body, err = io.ReadAll(io.LimitReader(reader, (1<<20)+1))
			if err != nil || len(body) > 1<<20 {
				return fmt.Errorf("editor output read or bound")
			}
			return nil
		})
	return body, err
}

func cursorEditorFrozen(t *testing.T, ctx context.Context, roots map[string]string) map[string]map[string]acpMetadataFile {
	t.Helper()
	result := map[string]map[string]acpMetadataFile{}
	for label, root := range roots {
		resolved, err := filepath.EvalSymlinks(root)
		if err != nil || resolved != root || !filepath.IsAbs(root) {
			t.Fatal("editor frozen root authority")
		}
		facts, err := cursorEditorTree(ctx, root)
		if err != nil {
			t.Fatalf("editor frozen tree refused: %s", label)
		}
		result[label] = facts
	}
	return result
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
	cursorEditorObserve(t, ctx, temp, output, report)
}

func cursorEditorObserve(t *testing.T, ctx context.Context, temp, output string, report map[string]any) {
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
	runtimeBefore := cursorEditorFrozen(t, ctx, map[string]string{"runtime": runtimeRoot})["runtime"]
	report["runtime_before_help"] = cursorEditorSummary(t, runtimeBefore)
	root, project, env, _ := acpMetadataProfile(t, "cursor")
	for _, key := range []string{"DISPLAY", "XAUTHORITY"} {
		if value := os.Getenv(key); value != "" {
			env = append(env, key+"="+value)
		}
	}
	cursorEditorHelpAndUI(t, ctx, root, project, output, source, paths, env, filesBefore, runtimeBefore, report)
}

func cursorEditorHelpAndUI(t *testing.T, ctx context.Context, root, project, output, source string, paths map[string]string, env []string, filesBefore, runtimeBefore map[string]acpMetadataFile, report map[string]any) {
	t.Helper()
	report["stage"] = "native-help"
	probe, stop := context.WithTimeout(ctx, 30*time.Second)
	help, helpErr := cursorEditorCapture(probe, []string{paths["launcher"], "--help"}, env, project, nil)
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
	runtimeAfterHelp := cursorEditorFrozen(t, ctx, map[string]string{"runtime": filepath.Dir(paths["electron"])})["runtime"]
	if !acpMetadataSame(runtimeBefore, runtimeAfterHelp) || !acpMetadataSame(filesBefore, cursorEditorFiles(t, paths)) {
		t.Fatal("editor help changed frozen acquisition")
	}
	input := cursorEditorInput(t, root, project, output, source, paths["electron"], help)
	roots := map[string]string{"runtime": filepath.Dir(paths["electron"]), "project-skills": filepath.Join(project, ".cursor"),
		"user-plugin":     filepath.Join(root, "home", ".cursor", "plugins", "local"),
		"playwright":      filepath.Join(source, "ui", "skills-picker", "node_modules", "playwright"),
		"playwright-core": filepath.Join(source, "ui", "skills-picker", "node_modules", "playwright-core")}
	before := cursorEditorFrozen(t, ctx, roots)
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
	observed, runErr := cursorEditorCapture(ctx, []string{paths["node"], paths["driver"]}, env, project, encoded)
	stdoutDigest := sha256.Sum256(observed)
	report["driver_stdout_bytes"], report["driver_stdout_sha256"] = len(observed), hex.EncodeToString(stdoutDigest[:])
	var result map[string]any
	decodeErr := json.Unmarshal(observed, &result)
	after, filesAfter := cursorEditorFrozen(t, ctx, roots), cursorEditorFiles(t, paths)
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
