package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/dirswap"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/environment"
)

const (
	cliCrashRootEnv  = "UAP_TEST_PUBLIC_CLI_DURABLE_CRASH_ROOT_20261104"
	cliCrashExitCode = 85
)

// The only crash hook is the existing test-only durable directory phase.
// Production composition and the new public CLI recovery process have no hooks.
func runPublicCLICrashChild(root string) {
	namespace := filepath.Join(root, ".uap")
	manager := dirswap.Manager{Namespace: namespace, JournalDir: filepath.Join(namespace, "operations-v2"),
		Fault: func(phase string) error {
			if phase == dirswap.PhaseBackupPending {
				os.Exit(cliCrashExitCode)
			}
			return nil
		}}
	_, err := manager.Apply(context.Background(), dirswap.Input{
		EnvironmentID:   environment.EnvironmentProjectDefault,
		OperationID:     "public-cli-crash-before-backup",
		ClientBindingID: "disposable-pending-operation",
		Sequence:        1,
		OwnedBase:       filepath.Join(namespace, "managed"),
		ActivePath:      filepath.Join(namespace, "managed", "unpublished-candidate"),
		StagingPath:     filepath.Join(namespace, "managed", ".unpublished-staging"),
		RequireAbsent:   true,
	})
	fmt.Fprintln(os.Stderr, "durable PhaseBackupPending exit was not reached:", err)
	os.Exit(76)
}

func projectCrashTestEnv(home, dataRoot, childRoot string) []string {
	env := []string{"HOME=" + home, "USERPROFILE=" + home, "AGENTPLUGINS_HOME=" + dataRoot}
	for _, key := range []string{"PATH", "SystemRoot", "SYSTEMROOT", "COMSPEC", "TMPDIR", "TEMP", "TMP", "GIT_CONFIG_NOSYSTEM"} {
		if value := os.Getenv(key); value != "" {
			env = append(env, key+"="+value)
		}
	}
	env = append(env, "APPDATA="+filepath.Join(home, "AppData", "Roaming"),
		"LOCALAPPDATA="+filepath.Join(home, "AppData", "Local"))
	if childRoot != "" {
		env = append(env, cliCrashRootEnv+"="+childRoot)
	}
	return env
}

func TestPublicCLICrashAfterDurablePhaseRecovery(t *testing.T) {
	if root := os.Getenv(cliCrashRootEnv); root != "" {
		runPublicCLICrashChild(root)
		return
	}
	// Build the genuine public executable before the fixture overrides HOME.
	// Otherwise Go downloads read-only modules into t.TempDir and Go's
	// test cleanup can fail on module-cache directory permissions.
	exe := filepath.Join(t.TempDir(), "agentplugins")
	if runtime.GOOS == "windows" {
		exe += ".exe"
	}
	build, cancelBuild := context.WithTimeout(t.Context(), 4*time.Minute)
	defer cancelBuild()
	cmd := exec.CommandContext(build, "go", "build", "-trimpath", "-o", exe, ".")
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build fresh public CLI: %v: %s", err, bytes.TrimSpace(output))
	}

	// Construct an owned project through the existing real TLS-Git/package
	// journey. Its inert Gemini detector is setup-only, never used by the
	// separate recovery binary.
	journey := newProjectJourney(t, newProjectJourneyTransport(t, "git"))
	journey.run(t, journey.a, "add", journey.transport.endpoint, "--alias", "docs", "--target", "gemini")
	originalState := projectJourneyRead(t, filepath.Join(journey.namespace(journey.a), "state-v2.json"))
	originalNative := securityFixtureTree(t, journey.nativeRoot(journey.a))
	staging := filepath.Join(journey.namespace(journey.a), "managed", ".unpublished-staging")
	projectJourneyWrite(t, filepath.Join(staging, "unpublished.txt"), "unpublished caller bytes\n")

	child := exec.CommandContext(t.Context(), os.Args[0], "-test.run=^TestPublicCLICrashAfterDurablePhaseRecovery$", "-test.count=1")
	child.Env = projectCrashTestEnv(journey.home, journey.cache, journey.a)
	output, childErr := child.CombinedOutput()
	var exit *exec.ExitError
	if !errors.As(childErr, &exit) || exit.ExitCode() != cliCrashExitCode {
		t.Fatalf("real OS child did not exit at durable phase: exit=%v, output=%q", childErr, output)
	}
	journal := filepath.Join(journey.namespace(journey.a), "operations-v2", "public-cli-crash-before-backup.json")
	var receipt dirswap.Receipt
	journalBytes, err := os.ReadFile(journal)
	if err != nil || json.Unmarshal(journalBytes, &receipt) != nil ||
		receipt.Phase != dirswap.PhaseBackupPending {
		t.Fatalf("crashed child did not leave a durable phase record: %v", err)
	}
	if body := projectJourneyRead(t, filepath.Join(staging, "unpublished.txt")); body != "unpublished caller bytes\n" {
		t.Fatal("pre-recovery caller-owned staging changed")
	}

	run := func(args ...string) []byte {
		t.Helper()
		ctx, cancel := context.WithTimeout(t.Context(), time.Minute)
		defer cancel()
		command := exec.CommandContext(ctx, exe, append(args, "--scope", "project", "--project-root", journey.a, "--format", "json")...)
		command.Env = projectCrashTestEnv(journey.home, journey.cache, "")
		command.Dir = journey.a
		stdout, err := command.Output()
		if err != nil {
			stderr := ""
			var exit *exec.ExitError
			if errors.As(err, &exit) {
				stderr = string(exit.Stderr)
				if len(stderr) > 4096 {
					stderr = stderr[:4096]
				}
			}
			t.Fatalf("fresh public CLI %s failed: %v; stderr=%q", strings.Join(args, " "), err, stderr)
		}
		return stdout
	}
	status := func() bool {
		t.Helper()
		var decoded struct {
			PendingRecovery bool `json:"pending_recovery"`
		}
		if err := json.Unmarshal(run("deps", "status"), &decoded); err != nil {
			t.Fatal(err)
		}
		return decoded.PendingRecovery
	}
	if !status() {
		t.Fatal("cold public CLI did not report pending recovery after real crash")
	}
	run("deps", "recover", "--yes")
	if status() {
		t.Fatal("second cold public CLI still reports pending recovery")
	}
	var repeated struct {
		Outcome string `json:"outcome"`
	}
	if err := json.Unmarshal(run("deps", "recover", "--yes"), &repeated); err != nil || repeated.Outcome != "unchanged" {
		t.Fatalf("repeated recovery must be idempotent: outcome=%q, error=%v", repeated.Outcome, err)
	}
	if _, err := os.Lstat(journal); !os.IsNotExist(err) {
		t.Fatalf("recovery left a live journal: %v", err)
	}
	if originalState != projectJourneyRead(t, filepath.Join(journey.namespace(journey.a), "state-v2.json")) {
		t.Fatal("cold CLI recovery rewrote committed project receipt")
	}
	if !reflect.DeepEqual(originalNative, securityFixtureTree(t, journey.nativeRoot(journey.a))) {
		t.Fatal("cold CLI recovery changed foreign/native project files")
	}
	if projectJourneyRead(t, filepath.Join(staging, "unpublished.txt")) != "unpublished caller bytes\n" {
		t.Fatal("cold CLI recovery adopted caller staging")
	}
}
