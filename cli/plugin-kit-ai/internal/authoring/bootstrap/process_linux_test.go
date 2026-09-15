//go:build linux

package bootstrap_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/777genius/plugin-kit-ai/cli/internal/authoring/bootstrap"
)

func TestDefaultRunnerExecutesOnlyLockedCommandWithSanitizedEnvironment(t *testing.T) {
	root, project := generated(t)
	plan, err := (bootstrap.Service{}).Plan(root, project)
	if err != nil {
		t.Fatal(err)
	}
	bin := t.TempDir()
	script := `#!/bin/sh
set -eu
test "$#" -eq 4
test "$1" = ci
test "$2" = --ignore-scripts
test "$3" = --no-audit
test "$4" = --no-fund
test -n "$HOME"
test -n "$NPM_CONFIG_CACHE"
test -n "$NPM_CONFIG_USERCONFIG"
test -n "$NPM_CONFIG_GLOBALCONFIG"
test -n "$TMPDIR"
test -z "${NPM_TOKEN:-}"
test -z "${HTTPS_PROXY:-}"
/bin/mkdir node_modules
`
	if err := os.WriteFile(filepath.Join(bin, "npm"), []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)
	t.Setenv("NPM_TOKEN", "must-not-reach-process")
	t.Setenv("HTTPS_PROXY", "https://must-not-reach-process.invalid")
	committed, err := (bootstrap.Service{}).Apply(context.Background(), root, plan)
	if err != nil || !committed {
		t.Fatalf("apply = %t, %v", committed, err)
	}

	failureRoot, failureProject := generated(t)
	failurePlan, err := (bootstrap.Service{}).Plan(failureRoot, failureProject)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bin, "npm"), []byte("#!/bin/sh\nexit 23\n"), 0700); err != nil {
		t.Fatal(err)
	}
	committed, err = (bootstrap.Service{}).Apply(context.Background(), failureRoot, failurePlan)
	if committed || errorCode(err) != "bootstrap_process_failed" {
		t.Fatalf("failed process = %t, %v", committed, err)
	}
	if _, err := os.Stat(filepath.Join(failureRoot, "node_modules")); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("failed process committed output")
	}
}
