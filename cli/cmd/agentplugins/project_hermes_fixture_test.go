package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"reflect"
	"regexp"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/atomicfile"
	processadapter "github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/process"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/clientdetect"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/hermes"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	legacyports "github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

type hermesColdInputs struct {
	bundle, uap, uapDigest, head, report string
	osHome, temporary, root              string
	ctx                                  context.Context
	protected                            map[string]map[string]string
	commands                             []map[string]any
}

type hermesColdRoot struct {
	receipt      domain.MutationReceipt
	root, parent os.FileInfo
	marker       string
}

func hermesColdRootAuthority(t *testing.T, journey projectJourney, binding domain.ClientBinding) hermesColdRoot {
	t.Helper()
	root := filepath.Join(journey.a, ".hermes", "uap-session")
	body, err := atomicfile.ReadBoundedRegularFile(filepath.Join(root, ".uap-owner"), 4096)
	if err != nil {
		t.Fatal("Hermes session root marker is missing, aliased or oversized", err)
	}
	marker := string(body)
	if marker != "uap-hermes-project-session-v1\n"+journey.a+"\n" {
		t.Fatal("Hermes session root marker differs from its selected Project")
	}
	digest := sha256.Sum256([]byte(marker))
	want := domain.NativeObjectOwnership{ObjectID: "hermes_project_session_root", Kind: "hermes_project_session_root",
		Path: filepath.Join(root, ".uap-owner"), ManagedDigest: hex.EncodeToString(digest[:]), ProtectionClass: "owned"}
	objects := 0
	for _, object := range binding.NativeObjects {
		if object.Kind == want.Kind {
			objects++
			if object != want {
				t.Fatal("Hermes root object differs from its exact owned marker")
			}
		}
	}
	if objects != 1 {
		t.Fatal("Hermes binding lacks one exact session root object")
	}
	result, receipts := hermesColdRoot{marker: marker}, 0
	for _, receipt := range journey.state(t, journey.a).TransactionReceipts {
		if strings.HasPrefix(receipt.AfterDigest, "native-root-v1:") {
			receipts++
			if receipt.ActivePath != root || receipt.MutationType != "directory_swap" || receipt.Phase != "committed" ||
				receipt.ClientBindingID != binding.ClientBindingID || receipt.OperationID == "" || receipt.Sequence != 1 ||
				!regexp.MustCompile("^native-root-v1:[0-9a-f]{64}$").MatchString(receipt.AfterDigest) {
				t.Fatal("Hermes root publication differs from the committed standalone authority")
			}
			result.receipt = receipt
		}
	}
	if receipts != 1 {
		t.Fatal("Hermes session root lacks one committed publication receipt")
	}
	for path, target := range map[string]*os.FileInfo{root: &result.root, filepath.Dir(root): &result.parent} {
		info, err := os.Lstat(path)
		if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			t.Fatal("Hermes retained session root or parent is missing or aliased", err)
		}
		*target = info
	}
	return result
}

func hermesColdSelection(t *testing.T) *hermesColdInputs {
	t.Helper()
	keys := []string{"UAP_HERMES_PROJECT_BUNDLE_E2E", "UAP_HERMES_PROJECT_BUNDLE_EXE", "UAP_HERMES_COLD_UAP_SHA256", "UAP_HERMES_COLD_REPORT", "UAP_HERMES_COLD_DEADLINE_UNIX_MS"}
	present := false
	for _, key := range keys {
		_, found := os.LookupEnv(key)
		present = present || found
	}
	if !present {
		t.Skip("real bundled Hermes cold lifecycle runs in the dedicated native workflow")
	}
	input := &hermesColdInputs{bundle: os.Getenv(keys[1]), uap: os.Getenv("UAP_BIN"),
		uapDigest: os.Getenv(keys[2]), head: os.Getenv("EXPECTED_HEAD"), report: os.Getenv(keys[3])}
	if os.Getenv(keys[0]) != "1" || os.Getenv("GITHUB_ACTIONS") != "true" || os.Getenv("RUNNER_ENVIRONMENT") != "github-hosted" ||
		!regexp.MustCompile("^[0-9a-f]{40}$").MatchString(input.head) ||
		!regexp.MustCompile("^[0-9a-f]{64}$").MatchString(input.uapDigest) {
		t.Fatal("partial or unqualified genuine Hermes cold lifecycle selection")
	}
	for _, path := range []string{input.bundle, input.uap, input.report} {
		if !filepath.IsAbs(path) || filepath.Clean(path) != path {
			t.Fatal("native cold lifecycle requires exact absolute selected paths")
		}
	}
	parent, err := filepath.EvalSymlinks(filepath.Dir(input.report))
	if err != nil || parent != filepath.Dir(input.report) {
		t.Fatal("native cold report parent is not an exact existing directory", err)
	}
	name := "hermes"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	if filepath.Base(input.bundle) != name || filepath.Base(filepath.Dir(input.bundle)) != "bin" {
		t.Fatal("native cold lifecycle requires the official bundled Hermes launcher")
	}
	hardDeadline, ok := t.Deadline()
	if !ok {
		t.Fatal("native cold lifecycle requires the fixed test timeout")
	}
	deadline, err := hermesColdDeadline(os.Getenv(keys[4]), time.Now(), hardDeadline)
	if err != nil {
		t.Fatal(err)
	}
	signals, stop := signal.NotifyContext(t.Context(), os.Interrupt, syscall.SIGTERM)
	bounded, cancel := context.WithDeadline(signals, deadline)
	input.ctx = bounded
	t.Cleanup(func() { cancel(); stop() })
	input.verifyInstaller(t)
	ctx, cancelProbe := context.WithTimeout(input.ctx, 15*time.Second)
	defer cancelProbe()
	result, err := (processadapter.OS{}).Run(ctx, legacyports.Command{Argv: []string{"git", "rev-parse", "HEAD"}, Env: os.Environ(), StdoutLimitBytes: 4096})
	if err != nil || result.ExitCode != 0 || !result.CleanupComplete || result.NoChildStarted || strings.TrimSpace(string(result.Stdout)) != input.head {
		t.Fatal("native cold lifecycle source differs from the workflow head", err)
	}
	if _, err := os.Lstat(input.report); !os.IsNotExist(err) {
		t.Fatal("native cold lifecycle report must be a fresh selected file")
	}
	t.Setenv("PATH", filepath.Dir(input.bundle)+string(os.PathListSeparator)+os.Getenv("PATH"))
	input.root = hermesColdDirectory(t)
	return input
}

func hermesColdDeadline(encoded string, now, hard time.Time) (time.Time, error) {
	if !regexp.MustCompile(`^[1-9]\d{0,18}$`).MatchString(encoded) {
		return time.Time{}, fmt.Errorf("native cold deadline is not exact positive Unix milliseconds")
	}
	milliseconds, err := strconv.ParseInt(encoded, 10, 64)
	if err != nil {
		return time.Time{}, fmt.Errorf("native cold deadline overflows: %w", err)
	}
	deadline := time.UnixMilli(milliseconds)
	if deadline.After(now.Add(6 * time.Minute)) {
		return time.Time{}, fmt.Errorf("native cold deadline exceeds its parent budget")
	}
	if hard.Before(deadline) {
		deadline = hard
	}
	deadline = deadline.Add(-30 * time.Second)
	if !deadline.After(now) {
		return time.Time{}, fmt.Errorf("native cold deadline lacks its cleanup reserve")
	}
	return deadline, nil
}

func hermesColdDirectory(t *testing.T) string {
	t.Helper()
	root := os.Getenv("TMPDIR")
	if root == "" || os.Getenv("TEMP") != root || os.Getenv("TMP") != root || !filepath.IsAbs(root) || filepath.Clean(root) != root {
		t.Fatal("native cold temporary root must be selected once by its caller")
	}
	physical, err := filepath.EvalSymlinks(root)
	if err != nil || physical != root {
		t.Fatal("native cold temporary root is absent or aliased", err)
	}
	entries, err := os.ReadDir(root)
	if err != nil || len(entries) != 0 {
		t.Fatal("native cold temporary root is not fresh", err)
	}
	// Only the caller may remove this tree after validating the full report and
	// successful test exit. Failures, canceled contexts and unknown children keep it.
	return root
}

func (input *hermesColdInputs) verifyInstaller(t *testing.T) {
	t.Helper()
	body, err := atomicfile.ReadBoundedRegularFile(input.uap, 128<<20)
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(body)
	if hex.EncodeToString(digest[:]) != input.uapDigest {
		t.Fatal("selected public installer differs from the exact source build receipt")
	}
}

func (input *hermesColdInputs) isolate(t *testing.T) {
	t.Helper()
	input.osHome = os.Getenv("HOME")
	input.temporary = (&projectJourneyTransport{tempRoot: input.root}).temporary(t)
	for _, key := range []string{"TMPDIR", "TEMP", "TMP"} {
		t.Setenv(key, input.temporary)
	}
	for key, relative := range map[string]string{
		"APPDATA": "AppData/Roaming", "LOCALAPPDATA": "AppData/Local",
		"XDG_CONFIG_HOME": "config", "XDG_CACHE_HOME": "cache",
		"XDG_DATA_HOME": "data", "XDG_STATE_HOME": "state",
	} {
		path := filepath.Join(input.osHome, filepath.FromSlash(relative))
		if err := os.MkdirAll(path, 0700); err != nil {
			t.Fatal(err)
		}
		t.Setenv(key, path)
	}
	// The selected User profile and the native default are distinct authorities.
	// Neither contains real credentials, and both are checked after each step.
	native := filepath.Join(input.osHome, ".hermes")
	if runtime.GOOS == "windows" {
		native = filepath.Join(os.Getenv("LOCALAPPDATA"), "hermes")
	}
	projectJourneyWrite(t, filepath.Join(native, "config.yaml"), "theme: retained-default-user\n")
	projectJourneyWrite(t, filepath.Join(native, "foreign.txt"), "default-user-sentinel\n")
	for _, key := range []string{"HERMES_HOME", "HERMES_REAL_HOME", "HERMES_DATA_DIR_SUFFIX"} {
		t.Setenv(key, "")
	}
	input.protected = map[string]map[string]string{
		input.osHome:    securityFixtureTree(t, input.osHome),
		input.temporary: securityFixtureTree(t, input.temporary),
	}
}

func newHermesColdJourney(t *testing.T, input *hermesColdInputs, transport *projectJourneyTransport) projectJourney {
	t.Helper()
	journey := newProjectJourney(t, transport)
	for _, project := range []string{journey.a, journey.b} {
		transport.git(t, project, "init", "-b", "main")
	}
	user := filepath.Join(journey.home, ".hermes")
	for _, path := range []string{user, filepath.Join(journey.a, ".hermes"), filepath.Join(journey.b, ".hermes")} {
		projectJourneyWrite(t, filepath.Join(path, "skills", "foreign", "SKILL.md"), "retained foreign Hermes skill\n")
	}
	projectJourneyWrite(t, filepath.Join(user, "config.yaml"), "theme: retained-selected-user\n")
	projectJourneyWrite(t, filepath.Join(journey.a, ".hermes", "foreign.txt"), "project-a-foreign\n")
	journey.app = configureHermesColdJourney(input.ctx, t, journey.app, user, input.bundle)
	input.protected[user] = securityFixtureTree(t, user)
	input.protected[journey.b] = securityFixtureTree(t, journey.b)
	for _, path := range []string{filepath.Join(journey.a, ".git"), filepath.Join(journey.a, ".hermes", "foreign.txt"),
		filepath.Join(journey.a, ".hermes", "skills", "foreign", "SKILL.md")} {
		input.protected[path] = securityFixtureTree(t, path)
	}
	return journey
}

func configureHermesColdJourney(ctx context.Context, t *testing.T, app agentpluginscli.App, user, executable string) agentpluginscli.App {
	t.Helper()
	registry, err := clients.NewRegistry(hermes.New())
	if err != nil {
		t.Fatal(err)
	}
	detector := clientdetect.NewOS(filepath.Dir(user))
	detector.Registry = registry
	detector.Environment["HERMES_HOME"] = user
	rows, err := detector.Detect(ctx)
	if err != nil || len(rows) != 1 || rows[0].ClientID != domain.ClientHermes ||
		rows[0].Status != domain.DetectionDetected || rows[0].ConfigRoot != user ||
		rows[0].ExecutablePath != executable || rows[0].DetectionError != nil {
		t.Fatalf("actual selected Hermes detector differs: %+v / %v", rows, err)
	}
	// Keep the real detector and all production session/process services after
	// every scope recomposition. Only the existing TLS/source/security fixture
	// boundaries are inherited from projectJourney.
	app.Detector = detector
	compose := app.ForScope
	app.ForScope = func(scope domain.InstallScope, root string) (agentpluginscli.App, error) {
		selected, err := compose(scope, root)
		if err != nil {
			return selected, err
		}
		return configureHermesColdJourney(ctx, t, selected, user, executable), nil
	}
	return app
}

func (input *hermesColdInputs) unchanged(t *testing.T) {
	t.Helper()
	for root, original := range input.protected {
		if !reflect.DeepEqual(original, securityFixtureTree(t, root)) {
			t.Fatal("native cold lifecycle changed an unselected User/Project B or temporary authority", root)
		}
	}
	input.verifyInstaller(t)
}

// Keep public Cobra's complete session verification in this process. Only the
// final StartNativeSession callback uses the ordinary runner instead of Unix
// exec, so cancellation cannot kill an intermediate owner of another group.
func (input *hermesColdInputs) inspect(t *testing.T, journey projectJourney, label string, expected map[string]any) {
	t.Helper()
	before := projectJourneyRead(t, filepath.Join(journey.a, ".uap", "state-v2.json"))
	config := filepath.Join(journey.a, ".hermes", "uap-session", "config.yaml")
	_, binding := journey.owned(t, journey.a)
	owned := map[string]map[string]string{}
	for _, path := range []string{config, binding.TargetLocator, filepath.Join(journey.a, ".hermes", "skills")} {
		owned[path] = securityFixtureTree(t, path)
	}
	publicArgs := []string{"run", "--target", "hermes", "--scope", "project",
		"--project-root", journey.a, "--inspect", "--confirm-client-start"}
	ctx, cancel := context.WithTimeout(input.ctx, 2*time.Minute)
	defer cancel()
	var result legacyports.CommandResult
	var argv []string
	var directory, selectedRoot string
	starts := 0
	start := func(ctx context.Context, plan clients.SessionPlan) error {
		starts++
		if starts != 1 {
			return fmt.Errorf("native cold session attempted another selected command")
		}
		argv, directory = append([]string{plan.Executable}, plan.Arguments...), plan.Directory
		env := plan.ChildEnvironment(os.Environ())
		roots := 0
		for _, entry := range env {
			key, value, _ := strings.Cut(entry, "=")
			if strings.EqualFold(key, "HERMES_HOME") {
				roots++
				selectedRoot = value
			}
		}
		if roots != 1 || selectedRoot != filepath.Join(journey.a, ".hermes", "uap-session") || directory != journey.a ||
			!slices.Equal(argv, []string{input.bundle, "config", "get", "mcp_servers", "--json"}) {
			return fmt.Errorf("native cold command or environment differs from its exact Project selection")
		}
		var err error
		result, err = (processadapter.OS{}).Run(ctx, legacyports.Command{Argv: argv, Env: env, Dir: directory, StdoutLimitBytes: 1 << 20})
		if !result.CleanupComplete || result.NoChildStarted {
			return fmt.Errorf("native cold selected command lacks completed cleanup")
		}
		return err
	}
	app := journey.app
	app.StartNativeSession = start
	compose := app.ForScope
	app.ForScope = func(scope domain.InstallScope, root string) (agentpluginscli.App, error) {
		selected, err := compose(scope, root)
		selected.StartNativeSession = start
		return selected, err
	}
	var output, diagnostics bytes.Buffer
	app.Input, app.Output, app.ErrorOutput, app.Terminal = strings.NewReader(""), &output, &diagnostics, false
	command := agentpluginscli.NewRoot(app)
	command.SetOut(&output)
	command.SetErr(&diagnostics)
	command.SetArgs(publicArgs)
	err := command.ExecuteContext(ctx)
	if err != nil || ctx.Err() != nil || starts != 1 || result.ExitCode != 0 || !result.CleanupComplete || result.NoChildStarted || len(result.Stderr) > 32768 {
		t.Fatal("actual public Cobra/native Hermes inspect did not complete; caller retains the fixture")
	}
	if directory != journey.a || selectedRoot != filepath.Join(journey.a, ".hermes", "uap-session") {
		t.Fatal("actual native command selected another Project authority")
	}
	stdoutDigest, stderrDigest := sha256.Sum256(result.Stdout), sha256.Sum256(result.Stderr)
	input.commands = append(input.commands, map[string]any{
		"step": label, "argv": argv, "public_argv": publicArgs, "directory": directory, "selected_root": selectedRoot, "exit_code": result.ExitCode,
		"stdout_base64": result.Stdout, "stderr_base64": result.Stderr,
		"stdout_bytes": len(result.Stdout), "stderr_bytes": len(result.Stderr),
		"stdout_sha256": hex.EncodeToString(stdoutDigest[:]), "stderr_sha256": hex.EncodeToString(stderrDigest[:]),
		"cleanup_complete": result.CleanupComplete, "no_child_started": result.NoChildStarted,
	})
	actual, err := shared.DecodeStrictJSONObject(result.Stdout)
	if err != nil || !reflect.DeepEqual(actual, expected) {
		t.Fatalf("actual native Hermes inventory differs from exact owned YAML: %v / %s", err, result.Stdout)
	}
	if projectJourneyRead(t, filepath.Join(journey.a, ".uap", "state-v2.json")) != before {
		t.Fatal("native inspection promoted or rewrote a Prepared receipt")
	}
	for path, original := range owned {
		if !reflect.DeepEqual(original, securityFixtureTree(t, path)) {
			t.Fatal("native inspection changed its owned source", path)
		}
	}
	input.unchanged(t)
}

func (input *hermesColdInputs) writeReport(t *testing.T, original, restored domain.Installation, binding domain.ClientBinding, publications []domain.MutationReceipt, removed domain.StateFileV2) {
	t.Helper()
	if err := input.ctx.Err(); err != nil {
		t.Fatal("native cold report refused after cancellation", err)
	}
	body, err := json.Marshal(map[string]any{
		"kind": "hermes-bundle-cold-restore", "source_head": input.head,
		"installer_sha256": input.uapDigest, "bundle_executable": input.bundle,
		"entrypoint": "public-cobra-in-process", "installer_executed": false,
		"runner_os": runtime.GOOS, "architecture": runtime.GOARCH,
		"original_source": original.Source, "restored_source": restored.Source,
		"restored_binding": binding, "commands": input.commands,
		"root_publications": publications, "removed_state": removed,
		"activation": "prepared", "native_tools_call": false,
		"vendor_authentication": false, "model_invoked": false,
	})
	if err != nil || len(body) > 1<<20 {
		t.Fatal("cold lifecycle report is not bounded JSON", err)
	}
	file, err := os.OpenFile(input.report, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		t.Fatal(err)
	}
	_, writeErr := file.Write(body)
	if closeErr := file.Close(); writeErr != nil || closeErr != nil {
		t.Fatalf("write native cold report: %v / %v", writeErr, closeErr)
	}
	if err := input.ctx.Err(); err != nil {
		t.Fatal("native cold report completed after cancellation; caller must retain the fixture", err)
	}
	t.Logf("HERMES_COLD_SOURCE %s %s (%d native selected commands)", input.head, input.uapDigest, len(input.commands))
}

func hermesColdSource(t *testing.T, transport *projectJourneyTransport, endpoint string) {
	t.Helper()
	body, err := json.Marshal(map[string]any{"$schema": domain.MCPSchemaV1,
		"mcpServers": map[string]any{"docs": map[string]any{"type": "streamable-http", "url": endpoint}}})
	if err != nil {
		t.Fatal(err)
	}
	projectJourneyWrite(t, filepath.Join(transport.repo, "mcp.json"), string(body))
	transport.git(t, transport.repo, "add", ".")
	transport.git(t, transport.repo, "commit", "-m", "Hermes owned HTTP fixture")
	transport.git(t, transport.repo, "push", "--quiet", transport.remote, "main")
}
