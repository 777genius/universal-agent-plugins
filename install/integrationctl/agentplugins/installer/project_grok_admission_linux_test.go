//go:build linux

package installer

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	legacyports "github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

// Reuse the original finite SDK protocol fixture through the readonly port.
// The production reader still captures and revalidates every original input.
type grokReadonlySDKNative struct{ *grokEnableSDKNative }

func (native grokReadonlySDKNative) RunReadonlyDuplexWithPlannedShutdown(ctx context.Context, command legacyports.Command, inputs legacyports.ReadonlyDuplexInputs, exchange func(io.Writer, io.Reader) error) error {
	if inputs.Home.Path != native.home || inputs.Project.Path != command.Dir || inputs.Worktree.Path != command.Dir || inputs.Executable.Path != native.executable {
		return fmt.Errorf("readonly SDK source roles differ")
	}
	return native.RunDuplexWithPlannedShutdown(ctx, command, exchange)
}

func admitGrokSDKFixture(t *testing.T, f *grokEnableSDKFixture) {
	t.Helper()
	deathWrite(t, filepath.Join(f.project, ".git", "config"), "[core]\nrepositoryformatversion=0\nbare=false\n")
	deathWrite(t, filepath.Join(f.project, ".git", "HEAD"), "ref: refs/heads/main\n")
	for _, directory := range []string{"objects", "refs"} {
		if err := os.MkdirAll(filepath.Join(f.project, ".git", directory), 0700); err != nil {
			t.Fatal(err)
		}
	}
	deathWrite(t, f.native.executable, "inert original native SDK input; not executed\n")
	if err := os.Chmod(f.native.executable, 0700); err != nil {
		t.Fatal(err)
	}
	f.engine.cfg.Runner = grokReadonlySDKNative{f.native}
}

func TestProjectGrokSDKContainedConsentEnableRepeatAndRemove(t *testing.T) {
	f := newGrokEnableSDKFixture(t, true)
	f.install(t)
	admitGrokSDKFixture(t, f)
	original := deathRead(t, f.config)
	request := f.selected(t, OpRepair, true)
	prepared, err := f.engine.Prepare(t.Context(), request)
	if err != nil {
		t.Fatal("contained explicit preview", err)
	}
	if f.native.duplex != 0 || deathRead(t, f.config) != original {
		t.Fatal("preview executed native inventory or mutated config")
	}
	if err := prepared.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := f.execute(t, request, Decision{Confirmed: true}); err != nil {
		t.Fatal("contained explicit repair", err)
	}
	enabled := f.binding(t)
	if enabled.NativeActivationAttempt != "" || enabled.Activation != domain.ActivationPrepared || len(enabled.NativeObjects) != 2 || f.native.duplex != 2 {
		t.Fatal("native enablement did not settle exact Prepared ownership", enabled, f.native.duplex)
	}
	after := deathRead(t, f.config)
	if _, err := f.execute(t, request, Decision{Confirmed: true}); err != nil {
		t.Fatal("repeated owned repair", err)
	}
	if deathRead(t, f.config) != after || f.binding(t).NativeObjects[1] != enabled.NativeObjects[1] {
		t.Fatal("repeat changed exact native member")
	}
	if _, err := f.execute(t, f.selected(t, OpRemove, false), Decision{Confirmed: true}); err != nil {
		t.Fatal("contained owned removal", err)
	}
	if deathRead(t, f.config) != original {
		t.Fatal("owned removal changed foreign config bytes")
	}
	if _, err := os.Stat(enabled.TargetLocator); !os.IsNotExist(err) {
		t.Fatal("owned directory remains", err)
	}
	view, err := f.engine.Inspect(t.Context())
	if err != nil || view.Recovery.Required {
		t.Fatal("completed native lifecycle left recovery work", err, view.Recovery)
	}
}
