package qoder

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	legacyports "github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

func TestQoderProfileResolutionAndInvalidExplicitDiscovery(t *testing.T) {
	base := t.TempDir()
	physical, err := filepath.EvalSymlinks(base)
	if err != nil {
		t.Fatal(err)
	}
	missing := filepath.Join(physical, "missing", "profile")
	if got, err := New().ResolveProfileRoot(missing); err != nil || got != missing {
		t.Fatalf("missing profile = %q, %v", got, err)
	}
	if _, err := os.Stat(filepath.Dir(missing)); !os.IsNotExist(err) {
		t.Fatalf("resolution created missing profile: %v", err)
	}
	file := filepath.Join(physical, "file")
	if err := os.WriteFile(file, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	for _, root := range []string{"", ".qoder", "relative/profile", " " + missing, missing + " ", missing + string(filepath.Separator), missing + "\n", file, filepath.Join(file, "child")} {
		if _, err := New().ResolveProfileRoot(root); err == nil {
			t.Errorf("invalid explicit profile accepted: %q", root)
		}
		if root == "" {
			continue // An absent environment override selects the home default.
		}
		probes := qoderHostProbes(physical, map[string]string{"QODER_CONFIG_DIR": root}, nil)
		if got := New().DetectSurfaces(clients.NewHost(probes)); got.Err == nil || got.ConfigRoot != "" || got.ExecutablePath != "" {
			t.Errorf("invalid override fell back to ambient profile: %q: %+v", root, got)
		}
	}
	alias := filepath.Join(physical, "alias")
	if err := os.Symlink(physical, alias); err != nil {
		t.Logf("symlink creation unavailable: %v", err)
		return
	}
	if got, err := New().ResolveProfileRoot(filepath.Join(alias, "missing", "profile")); err != nil || got != missing {
		t.Fatalf("profile alias = %q, %v", got, err)
	}
}

func qoderHostProbes(home string, environment map[string]string, binaries map[string]string) clients.HostProbes {
	return clients.HostProbes{HomeDir: home, GOOS: "linux", Environment: environment,
		Lstat: os.Lstat, ReadDir: os.ReadDir, EvalSymlinks: filepath.EvalSymlinks,
		LookPath: func(name string) (string, error) {
			if path := binaries[name]; path != "" {
				return path, nil
			}
			return "", fmt.Errorf("TEST absent")
		}}
}

func TestQoderDetectionUsesSelectedProfileAndBothCLICommandNames(t *testing.T) {
	home, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	profile := filepath.Join(home, "selected profile")
	if err := os.Mkdir(profile, 0o700); err != nil {
		t.Fatal(err)
	}
	primary, fallback := filepath.Join(home, "bin", "qoder"), filepath.Join(home, "bin", "qodercli")
	for _, tc := range []struct {
		name     string
		binaries map[string]string
		want     string
	}{{"primary", map[string]string{"qoder": primary, "qodercli": fallback}, primary}, {"fallback", map[string]string{"qodercli": fallback}, fallback}} {
		t.Run(tc.name, func(t *testing.T) {
			probes := qoderHostProbes(home, map[string]string{"QODER_CONFIG_DIR": profile, "QODER_HOME": filepath.Join(home, "ignored")}, tc.binaries)
			got := New().DetectSurfaces(clients.NewHost(probes))
			if got.Err != nil || got.ConfigRoot != profile || got.ExecutablePath != tc.want || !got.Surfaces[0].Detected || !got.Surfaces[1].Detected {
				t.Fatalf("selected discovery = %+v", got)
			}
		})
	}
}

func TestQoderNativeOperationsRejectInvalidOrDifferentProfileBeforeDispatch(t *testing.T) {
	r := qoderRequest(t)
	for _, root := range []string{"", "relative", r.Client.ConfigRoot + string(filepath.Separator)} {
		f := &fakeQoder{}
		request := r
		request.Client.ConfigRoot = root
		if _, err := New().Activate(t.Context(), clients.Env{Runner: f}, request); err == nil {
			t.Errorf("activation accepted %q", root)
		}
		if _, err := New().Deactivate(t.Context(), clients.Env{Runner: f}, domain.DeactivationRequest{Client: request.Client, DeclaredName: request.DeclaredName, BackendExecutable: request.BackendExecutable, Confirmed: true}); err == nil {
			t.Errorf("removal accepted %q", root)
		}
		if _, err := New().InspectNativeRegistry(t.Context(), clients.Env{Runner: f}, request.Client, request.Plan, nil); err == nil {
			t.Errorf("inspection accepted %q", root)
		}
		if len(f.commands) != 0 {
			t.Fatalf("invalid profile dispatched CLI: %v", f.commands)
		}
	}
	f := &fakeQoder{}
	r.Plan.NativeRegistryRoot = filepath.Join(t.TempDir(), "different")
	if _, err := New().Activate(t.Context(), clients.Env{Runner: f}, r); err == nil || len(f.commands) != 0 {
		t.Fatal("planned profile mismatch reached CLI")
	}
}

func TestQoderUserBindingCannotMoveToAnotherProfile(t *testing.T) {
	root, err := New().ResolveProfileRoot(filepath.Join(t.TempDir(), "selected"))
	if err != nil {
		t.Fatal(err)
	}
	binding := domain.ClientBinding{ClientID: "qoder", Scope: "user", NativeProfileRoot: root}
	if err := New().ValidateBindingProfile(root, binding); err != nil {
		t.Fatalf("original profile refused: %v", err)
	}
	if got, err := New().VersionProbeEnvironment(root); err != nil || !reflect.DeepEqual(got, []string{"QODER_CONFIG_DIR=" + root}) {
		t.Fatalf("profile-bound version environment = %v, %v", got, err)
	}
	for _, recorded := range []string{"", filepath.Join(filepath.Dir(root), "different")} {
		binding.NativeProfileRoot = recorded
		if err := New().ValidateBindingProfile(root, binding); err == nil {
			t.Fatalf("unproved or different recorded profile accepted: %q", recorded)
		}
	}
}

// Retain the real command environment and native arguments while using this
// test executable as an inert Qoder subprocess. The fixture writes only the
// profile selected by those arguments, just as a profile-aware native CLI does.
type profileProcessRunner struct{ commands []legacyports.Command }

func (r *profileProcessRunner) Run(ctx context.Context, command legacyports.Command) (legacyports.CommandResult, error) {
	r.commands = append(r.commands, command)
	argv := append([]string{"-test.run=^TestQoderProfileProcessFixture$", "--"}, command.Argv[1:]...)
	child := exec.CommandContext(ctx, command.Argv[0], argv...)
	child.Env, child.Dir = command.Env, command.Dir
	body, err := child.Output()
	result := legacyports.CommandResult{Stdout: body}
	var exited *exec.ExitError
	if errors.As(err, &exited) {
		result.ExitCode, result.Stderr = exited.ExitCode(), exited.Stderr
		return result, nil
	}
	return result, err
}

func TestQoderProfileProcessFixture(t *testing.T) {
	index := -1
	for i, arg := range os.Args {
		if arg == "--" {
			index = i
			break
		}
	}
	if index < 0 {
		return
	}
	args := os.Args[index+1:]
	if len(args) < 4 || args[0] != "--config-dir" || args[2] != "plugins" || os.Getenv("QODER_CONFIG_DIR") != args[1] || os.Getenv("QODER_HOME") != "" {
		os.Exit(71)
	}
	path := filepath.Join(args[1], "TEST-native-registry.json")
	if err := os.MkdirAll(args[1], 0o700); err != nil {
		os.Exit(72)
	}
	switch args[3] {
	case "list":
		body, err := os.ReadFile(path)
		if os.IsNotExist(err) {
			body, err = []byte("[]"), nil
		}
		if err != nil {
			os.Exit(73)
		}
		_, _ = os.Stdout.Write(body)
	case "install", "uninstall":
		if len(args) < 7 || args[len(args)-2] != "--scope" || args[len(args)-1] != "user" {
			os.Exit(74)
		}
		body := []byte(`[{"id":"demo@local","version":"2.0.0"}]`)
		if args[3] == "uninstall" {
			body = []byte("[]")
		}
		if err := os.WriteFile(path, body, 0o600); err != nil {
			os.Exit(75)
		}
	default:
		os.Exit(76)
	}
	os.Exit(0)
}

func TestQoderSelectedProfileLifecycleLeavesAmbientDefaultUntouched(t *testing.T) {
	base, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defaultRoot := filepath.Join(base, "ambient home", ".qoder")
	if err := os.MkdirAll(defaultRoot, 0o700); err != nil {
		t.Fatal(err)
	}
	defaultPath := filepath.Join(defaultRoot, "TEST-native-registry.json")
	defaultBody := []byte(`[{"id":"foreign@local","version":"1.0.0"}]`)
	if err := os.WriteFile(defaultPath, defaultBody, 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", filepath.Dir(defaultRoot))
	t.Setenv("USERPROFILE", filepath.Dir(defaultRoot))
	t.Setenv("QODER_CONFIG_DIR", defaultRoot)
	t.Setenv("QODER_HOME", filepath.Join(base, "another ambient profile"))
	r := qoderRequest(t)
	r.Client.ConfigRoot = filepath.Join(base, "selected profile")
	r.Plan.NativeRegistryRoot = r.Client.ConfigRoot
	r.BackendExecutable, err = os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	r.Plan.NativeRegistryExecutable = r.BackendExecutable
	f := &profileProcessRunner{}
	env := clients.Env{Runner: f}
	if finding, err := New().InspectNativeRegistry(t.Context(), env, r.Client, r.Plan, nil); err != nil || finding != clients.RegistryClear {
		t.Fatalf("selected profile inspection = %v, %v", finding, err)
	}
	if out, err := New().Activate(t.Context(), env, r); err != nil || out.Verification != domain.VerificationInstalled {
		t.Fatalf("install = %+v, %v", out, err)
	}
	r.VerifyOnly = true
	before := len(f.commands)
	if out, err := New().Activate(t.Context(), env, r); err != nil || out.Verification != domain.VerificationInstalled || len(f.commands) != before+1 {
		t.Fatalf("read-only verification = %+v, %v", out, err)
	}
	if finding, err := New().InspectNativeRegistry(t.Context(), env, r.Client, r.Plan, &domain.ClientBinding{ClientID: "qoder", NativeProfileRoot: r.Client.ConfigRoot}); err != nil || finding != clients.RegistryExpected {
		t.Fatalf("managed inspection = %v, %v", finding, err)
	}
	r.VerifyOnly, r.Replacing = false, true
	if out, err := New().Activate(t.Context(), env, r); err != nil || out.Verification != domain.VerificationInstalled {
		t.Fatalf("update = %+v, %v", out, err)
	}
	if out, err := New().Deactivate(t.Context(), env, domain.DeactivationRequest{Client: r.Client, DeclaredName: "demo", BackendExecutable: r.BackendExecutable, Confirmed: true}); err != nil || !out.ExternalRemovalComplete {
		t.Fatalf("remove = %+v, %v", out, err)
	}
	for _, command := range f.commands {
		if len(command.Argv) < 5 || !reflect.DeepEqual(command.Argv[1:3], []string{"--config-dir", r.Client.ConfigRoot}) {
			t.Fatalf("unbound native operation: %v", command.Argv)
		}
		for _, entry := range command.Env {
			if strings.HasPrefix(entry, "QODER_CONFIG_DIR=") && entry != "QODER_CONFIG_DIR="+r.Client.ConfigRoot {
				t.Fatalf("ambient profile leaked: %q", entry)
			}
		}
	}
	got, err := os.ReadFile(defaultPath)
	if err != nil || !reflect.DeepEqual(got, defaultBody) {
		t.Fatalf("ambient default profile changed: %q, %v", got, err)
	}
	got, err = os.ReadFile(filepath.Join(r.Client.ConfigRoot, "TEST-native-registry.json"))
	var entries []pluginEntry
	if err != nil || json.Unmarshal(got, &entries) != nil || len(entries) != 0 {
		t.Fatalf("selected profile removal = %q, %v", got, err)
	}
}
