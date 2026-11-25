package clientdetect

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/copilot"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/opencode"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func TestOpenCodeDetectorVersionProbeUsesDisposableProfile(t *testing.T) {
	binary := nativeOpenCodeFixture(t, "1.19.4", "isolated-ok")
	base, err := filepath.EvalSymlinks(filepath.Dir(binary))
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("TMP", base)
	t.Setenv("TEMP", base)
	home := filepath.Join(base, "selected-home")
	if err := os.Mkdir(home, 0700); err != nil {
		t.Fatal(err)
	}
	canary := filepath.Join(home, "foreign")
	if err := os.WriteFile(canary, []byte("unchanged"), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("OPENCODE_CONFIG_CONTENT", "parent-secret")
	t.Setenv("AGENTPLUGINS_TEST_CREDENTIAL", "parent-secret")
	d := NewOS(home)
	d.Registry, err = clients.NewRegistry(opencode.New())
	if err != nil {
		t.Fatal(err)
	}
	d.LookPath = func(name string) (string, error) {
		if name == "opencode" {
			return binary, nil
		}
		return "", errors.New("absent")
	}
	got, err := d.DetectTargetsWithVersionProbe(context.Background(), []domain.ClientID{domain.ClientOpenCode})
	if err != nil || len(got) != 1 || got[0].Version != "1.19.4" {
		t.Fatalf("detection = %+v, %v", got, err)
	}
	observed := readIsolatedProbeObservation(t, binary)
	if len(observed.Failures) != 0 || strings.TrimSpace(observed.Config) != "{}" {
		t.Fatalf("unsafe probe: %+v", observed)
	}
	for _, entry := range observed.Env {
		if strings.Contains(entry, "parent-secret") || strings.Contains(entry, home) {
			t.Fatalf("probe inherited selected profile or secret: %s", entry)
		}
	}
	privateHome := observed.Roots["HOME"]
	if privateHome == "" || !strings.HasPrefix(privateHome, base+string(os.PathSeparator)) {
		t.Fatalf("not a private HOME: %q", privateHome)
	}
	if _, err := os.Stat(filepath.Dir(privateHome)); !os.IsNotExist(err) {
		t.Fatalf("private probe root not removed: %v", err)
	}
	if body, err := os.ReadFile(canary); err != nil || string(body) != "unchanged" {
		t.Fatalf("selected profile changed: %q, %v", body, err)
	}
}

func TestOpenCodeDetectorCleansDisposableProfileAfterFailureOrCancellation(t *testing.T) {
	for _, cancelProbe := range []bool{false, true} {
		t.Run(map[bool]string{false: "failure", true: "cancellation"}[cancelProbe], func(t *testing.T) {
			base, err := filepath.EvalSymlinks(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			t.Setenv("TMPDIR", base)
			t.Setenv("TMP", base)
			t.Setenv("TEMP", base)
			home := filepath.Join(base, "selected")
			if err := os.Mkdir(home, 0700); err != nil {
				t.Fatal(err)
			}
			d := NewOS(home)
			d.Registry, err = clients.NewRegistry(opencode.New())
			if err != nil {
				t.Fatal(err)
			}
			d.LookPath = func(name string) (string, error) {
				if name == "opencode" {
					return filepath.Join(base, "native"), nil
				}
				return "", errors.New("absent")
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			var private string
			d.ProbeVersionWithEnvironment = func(_ context.Context, _ string, env []string) (string, error) {
				for _, entry := range env {
					if value, ok := strings.CutPrefix(entry, "HOME="); ok {
						private = filepath.Dir(value)
					}
				}
				if private == "" {
					t.Fatal("probe received no private HOME")
				}
				if cancelProbe {
					cancel()
					return "", context.Canceled
				}
				return "", errors.New("injected failure")
			}
			_, _ = d.DetectTargetsWithVersionProbe(ctx, []domain.ClientID{domain.ClientOpenCode})
			if private == "" {
				t.Fatal("version probe did not run")
			}
			if _, err := os.Stat(private); !os.IsNotExist(err) {
				t.Fatalf("private profile remains after failure: %v", err)
			}
		})
	}
}

func TestOpenCodeDetectorVersionProbeAcceptsAliasedConfigRoot(t *testing.T) {
	base, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	scratch := filepath.Join(base, "scratch")
	home := filepath.Join(base, "home")
	configParent := filepath.Join(home, ".config")
	target := filepath.Join(base, "selected-config")
	for _, dir := range []string{scratch, configParent, target} {
		if err := os.MkdirAll(dir, 0700); err != nil {
			t.Fatal(err)
		}
	}
	alias := filepath.Join(configParent, "opencode")
	if err := os.Symlink(target, alias); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	canary := filepath.Join(target, "foreign")
	if err := os.WriteFile(canary, []byte("unchanged"), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TMPDIR", scratch)
	t.Setenv("TMP", scratch)
	t.Setenv("TEMP", scratch)
	t.Setenv("XDG_CONFIG_HOME", configParent)
	d := NewOS(home)
	d.Registry, err = clients.NewRegistry(opencode.New())
	if err != nil {
		t.Fatal(err)
	}
	d.LookPath = func(name string) (string, error) {
		if name == "opencode" {
			return filepath.Join(base, "native"), nil
		}
		return "", errors.New("absent")
	}
	var private string
	d.ProbeVersionWithEnvironment = func(_ context.Context, _ string, env []string) (string, error) {
		for _, entry := range env {
			if value, ok := strings.CutPrefix(entry, "HOME="); ok {
				private = filepath.Dir(value)
			}
		}
		return "1.19.4", nil
	}
	got, err := d.DetectTargetsWithVersionProbe(context.Background(), []domain.ClientID{domain.ClientOpenCode})
	if err != nil || len(got) != 1 || got[0].Status != domain.DetectionDetected || got[0].Version != "1.19.4" || got[0].ConfigRoot != alias {
		t.Fatalf("aliased OpenCode detection = %+v, %v", got, err)
	}
	if private == "" {
		t.Fatal("native probe did not receive a private profile")
	}
	if _, err := os.Stat(private); !os.IsNotExist(err) {
		t.Fatalf("private profile remains: %v", err)
	}
	if body, err := os.ReadFile(canary); err != nil || string(body) != "unchanged" {
		t.Fatalf("selected config changed: %q, %v", body, err)
	}
}

func TestOpenCodeVersionProbeRejectsTempOverlapThroughAlias(t *testing.T) {
	base, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(base, "selected-config")
	scratch := filepath.Join(target, "scratch")
	if err := os.MkdirAll(scratch, 0700); err != nil {
		t.Fatal(err)
	}
	alias := filepath.Join(base, "config-alias")
	if err := os.Symlink(target, alias); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	t.Setenv("TMPDIR", scratch)
	t.Setenv("TMP", scratch)
	t.Setenv("TEMP", scratch)
	adapter := opencode.New()
	env, cleanup, err := versionProbeEnvironment(adapter, adapter, alias)
	defer cleanup()
	if err == nil || len(env) != 0 {
		t.Fatalf("overlapping profile accepted: env=%v err=%v", env, err)
	}
	entries, err := os.ReadDir(scratch)
	if err != nil || len(entries) != 0 {
		t.Fatalf("overlapping scratch modified: %v %v", entries, err)
	}
}

func TestCopilotVersionProbeStillRejectsAliasedConfigRoot(t *testing.T) {
	base, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(base, "selected-config")
	if err := os.Mkdir(target, 0700); err != nil {
		t.Fatal(err)
	}
	alias := filepath.Join(base, "config-alias")
	if err := os.Symlink(target, alias); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	adapter := copilot.New()
	env, cleanup, err := versionProbeEnvironment(adapter, adapter, alias)
	defer cleanup()
	if err == nil || len(env) != 0 {
		t.Fatalf("aliased Copilot profile accepted: env=%v err=%v", env, err)
	}
	entries, err := os.ReadDir(target)
	if err != nil || len(entries) != 0 {
		t.Fatalf("selected profile modified: %v %v", entries, err)
	}
}
