package commands_test

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"

	"github.com/777genius/plugin-kit-ai/cli/internal/authoring/scaffold"
)

func generatedBootstrapRoot(t *testing.T) string {
	t.Helper()
	root := filepath.Join(t.TempDir(), "generated")
	plan, err := scaffold.BuildPlan(scaffold.Options{Template: scaffold.MCPStdio, Name: "demo", Description: "Disposable command fixture.", Runtime: "node"})
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range plan.Files() {
		path := filepath.Join(root, filepath.FromSlash(file.Path))
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, file.Bytes, file.Mode); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func TestPublicBootstrapExposurePlanApplyAndFailure(t *testing.T) {
	a := publicApp(t)
	a.Bootstrap = true
	root := generatedBootstrapRoot(t)
	before := tree(t, root)
	calls := 0
	a.BootstrapRunner = func(_ context.Context, name string, args []string, dir string, _ []string) error {
		calls++
		if name != "npm" || !reflect.DeepEqual(args, []string{"ci", "--ignore-scripts", "--no-audit", "--no-fund"}) {
			t.Fatalf("process = %s %v", name, args)
		}
		return os.Mkdir(filepath.Join(dir, "node_modules"), 0700)
	}
	e, code, _ := publicRun(t, a, []string{"bootstrap", root, "--dry-run", "--format=json"}, false)
	if code != 0 || calls != 0 || e.Data.Requested.Mode != "read" || e.Data.Effects.Committed || e.Data.Bootstrap == nil || !e.Data.Bootstrap.Planned {
		t.Fatalf("plan = %+v, calls=%d", e, calls)
	}
	if e.Data.Bootstrap.Runtime != "node" || e.Data.Bootstrap.Manager != "npm" || !reflect.DeepEqual(e.Data.Bootstrap.Argv, []string{"npm", "ci", "--ignore-scripts", "--no-audit", "--no-fund"}) {
		t.Fatalf("sanitized JSON plan = %+v", e.Data.Bootstrap)
	}
	if !reflect.DeepEqual(before, tree(t, root)) {
		t.Fatal("dry-run changed project")
	}
	human := executeRaw(a, []string{"bootstrap", root, "--dry-run", "--format=human"}, false)
	if human.err != nil || !bytes.Contains(human.out, []byte("bootstrap: runtime node; manager npm; argv npm ci --ignore-scripts --no-audit --no-fund; planned true")) {
		t.Fatalf("human plan = %v %s", human.err, human.out)
	}
	e, code, _ = publicRun(t, a, []string{"bootstrap", root, "--format=json"}, true)
	if runtime.GOOS != "linux" {
		if code != 1 || calls != 0 || e.Data.Error == nil || e.Data.Error.Code != "bootstrap_platform_unsupported" || e.Data.Effects.Committed {
			t.Fatalf("unsupported apply = %+v, calls=%d", e, calls)
		}
		if !reflect.DeepEqual(before, tree(t, root)) {
			t.Fatal("unsupported apply changed project")
		}
	} else {
		if code != 0 || calls != 1 || e.Data.Requested.Mode != "local_mutation" || !e.Data.Effects.Committed || !reflect.DeepEqual(e.Data.Paths, []string{"node_modules"}) {
			t.Fatalf("apply = %+v, calls=%d", e, calls)
		}

		failureRoot := generatedBootstrapRoot(t)
		a.BootstrapRunner = func(context.Context, string, []string, string, []string) error {
			return errors.New("fixture process failure")
		}
		e, code, _ = publicRun(t, a, []string{"bootstrap", failureRoot, "--format=json"}, false)
		if code != 1 || e.Data.Error == nil || e.Data.Error.Code != "bootstrap_process_failed" || e.Data.Effects.Committed {
			t.Fatalf("failure = %+v", e)
		}
		if _, err := os.Stat(filepath.Join(failureRoot, "node_modules")); !errors.Is(err, os.ErrNotExist) {
			t.Fatal("failure committed dependencies")
		}
	}

	e, code, _ = publicRun(t, a, []string{"capabilities", "--format=json"}, false)
	if code != 0 || !contains(e.Data.Capabilities.Commands, "author.bootstrap") {
		t.Fatalf("bootstrap missing from capabilities: %+v", e.Data.Capabilities)
	}
}

func contains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
