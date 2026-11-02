package sourceacquisition

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/sourcespec"
)

type packageFailureRunner struct {
	tree    []byte
	failure error
	cancel  context.CancelFunc
}

func (runner packageFailureRunner) Run(_ context.Context, command Command) ([]byte, error) {
	if len(command.Args) > 2 {
		switch command.Args[2] {
		case "fetch":
			return nil, runner.failure
		case "rev-parse":
			return []byte(strings.Repeat("1", 40) + "\n"), nil
		case "ls-tree":
			if runner.cancel != nil {
				runner.cancel()
			}
			return runner.tree, nil
		}
	}
	return nil, nil
}

func TestDirectGitControlledPackageDiagnosticsAndRedaction(t *testing.T) {
	const secret = "credential-token-DO-NOT-PRINT"
	source := sourcespec.Git{URL: "https://git.test/team/repo.git", Path: "plugin"}
	for _, tc := range []struct{ name, tree, want string }{
		{"missing", "", "does not exist"},
		{"file", "100644 blob abc\tplugin\x00", "a file"},
		{"executable", "100755 blob abc\tplugin\x00", "executable file"},
		{"symlink", "120000 blob abc\tplugin\x00", "symbolic link"},
		{"submodule", "160000 commit abc\tplugin/vendor\x00", "submodule"},
		{"outside", "100755 blob abc\toutside/" + secret + "\x00", "outside the plugin subpath"},
		{"malformed", secret + "\x00", "invalid package tree"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			temporary := t.TempDir()
			_, err := (Acquirer{TempRoot: temporary, Runner: packageFailureRunner{tree: []byte(tc.tree)}}).AcquireGit(context.Background(), source, strings.Repeat("1", 40), "")
			if err == nil || !strings.Contains(err.Error(), tc.want) || !strings.Contains(err.Error(), `package path "plugin"`) || strings.Contains(err.Error(), secret) || strings.Contains(err.Error(), "credentials") {
				t.Fatalf("controlled diagnostic missing or leaked: %v", err)
			}
			archiveAssertEmpty(t, temporary)
		})
	}
	_, err := (Acquirer{TempRoot: t.TempDir(), Runner: packageFailureRunner{failure: errors.New(secret)}}).AcquireGit(context.Background(), source, strings.Repeat("1", 40), "")
	if err == nil || strings.Contains(err.Error(), secret) || !strings.Contains(err.Error(), "credentials") {
		t.Fatalf("transport error leaked or was path-classified: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	_, err = (Acquirer{TempRoot: t.TempDir(), Runner: packageFailureRunner{tree: []byte(secret), cancel: cancel}}).AcquireGit(ctx, source, strings.Repeat("1", 40), "")
	if !errors.Is(err, context.Canceled) || strings.Contains(err.Error(), secret) {
		t.Fatalf("cancellation lost to diagnostic: %v", err)
	}
}

func TestGitHubPackageDiagnosticKeepsPublicWording(t *testing.T) {
	_, err := (Acquirer{TempRoot: t.TempDir(), Runner: packageFailureRunner{}}).AcquireGitHub(context.Background(), "example/plugin", strings.Repeat("1", 40), "plugin")
	want := `acquire GitHub repository "example/plugin" at revision ` + strings.Repeat("1", 40) + `: inspect package tree: plugin subpath "plugin" does not exist`
	if err == nil || err.Error() != want {
		t.Fatalf("GitHub public wording changed: %v", err)
	}
}
