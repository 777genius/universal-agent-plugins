package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
)

// Run the real composition and managed process boundary in a disposable child:
// Unix replaces that child, while Windows retains its ordinary owned Job path.
func TestSessionCompositionUsesSuppliedEnvironment(t *testing.T) {
	for _, mode := range []string{"supplied", "empty"} {
		t.Run(mode, func(t *testing.T) {
			executable, err := os.Executable()
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
			defer cancel()
			command := exec.CommandContext(ctx, executable, "-test.run=^TestSessionEnvironmentEntry$", "--", "uap-session-entry", mode)
			command.Dir, command.WaitDelay = t.TempDir(), 2*time.Second
			ambient := clients.SessionPlan{Environment: map[string]string{
				"UAP_NATIVE_SESSION_VALUE": "ambient", "UAP_NATIVE_SESSION_AMBIENT": "must-not-leak", "HERMES_HOME": "foreign"}}
			command.Env = ambient.ChildEnvironment(os.Environ())
			output, err := command.CombinedOutput()
			if err != nil || ctx.Err() != nil || !strings.Contains(string(output), "SESSION_FINAL_ENV_OK") {
				t.Fatalf("actual selected environment failed: %v / %v / %s", err, ctx.Err(), output)
			}
		})
	}
}

func TestSessionEnvironmentEntry(t *testing.T) {
	if len(os.Args) < 3 || os.Args[len(os.Args)-2] != "uap-session-entry" {
		return
	}
	mode := os.Args[len(os.Args)-1]
	base := []string{}
	if mode == "supplied" {
		base = []string{"UAP_NATIVE_SESSION_VALUE=supplied", "UAP_NATIVE_SESSION_KEEP=original"}
	} else if mode != "empty" {
		t.Fatal("unknown handoff fixture mode")
	}
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	directory, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	app := agentpluginscli.App{}
	configureNativeSession(&app, nil, nativeconfig.New())
	plan := clients.SessionPlan{Executable: executable, Directory: directory,
		Arguments:   []string{"-test.run=^TestSessionEnvironmentFinal$", "--", "uap-session-final", mode},
		Environment: map[string]string{"HERMES_HOME": "selected-owned-project"}}
	if err := app.StartNativeSession(t.Context(), plan, base); err != nil {
		t.Fatal(err)
	}
}

func TestSessionEnvironmentFinal(t *testing.T) {
	if len(os.Args) < 3 || os.Args[len(os.Args)-2] != "uap-session-final" {
		return
	}
	mode := os.Args[len(os.Args)-1]
	value, keep := "", ""
	if mode == "supplied" {
		value, keep = "supplied", "original"
	} else if mode != "empty" {
		t.Fatal("unknown final fixture mode")
	}
	if os.Getenv("UAP_NATIVE_SESSION_VALUE") != value || os.Getenv("UAP_NATIVE_SESSION_KEEP") != keep ||
		os.Getenv("UAP_NATIVE_SESSION_AMBIENT") != "" || os.Getenv("HERMES_HOME") != "selected-owned-project" {
		t.Fatal("actual native start inherited an unverified environment")
	}
	fmt.Println("SESSION_FINAL_ENV_OK")
}
