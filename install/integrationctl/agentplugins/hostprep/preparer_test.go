package hostprep

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/clientdetect"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/opencodehost"
)

// RED if extraction aliases mutable target/environment/profile/selection data,
// infers authority from detected version text or forgets missing-root ancestors.
func TestSharedOpenCodeSnapshotIsolationAndRootFence(t *testing.T) {
	root := t.TempDir()
	a := filepath.Join(root, "a")
	b := filepath.Join(root, "b")
	link := filepath.Join(root, "selected")
	for _, dir := range []string{a, b} {
		if err := os.Mkdir(dir, 0700); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Symlink(a, link); err != nil {
		t.Fatal(err)
	}
	environment := []string{"PATH="}
	calls := 0
	probe := func(_ context.Context, target clientdetect.ProbeTarget) (clientdetect.ProbeEvidence, error) {
		calls++
		if target.Environment[0] != "PATH=" {
			t.Fatal("snapshot environment aliased")
		}
		target.Environment[0] = "PATH=changed-by-probe"
		return clientdetect.ProbeEvidence{VersionEvidence: opencodehost.VersionEvidence{Version: "2.0.21", Source: "executable_version", ProbeStatus: "ok", ExecutableIdentity: "TEST-pinned"}}, nil
	}
	preparer, err := New(probe, environment)
	if err != nil {
		t.Fatal(err)
	}
	environment[0] = "PATH=changed-by-caller"
	client := domain.DetectedClient{ClientID: domain.ClientOpenCode, ConfigRoot: filepath.Join(link, "missing", "opencode"), ExecutablePath: filepath.Join(root, "unlaunched"), Version: "99.0.0"}
	envelope := domain.PackageEnvelope{Skills: map[string]domain.Skill{"docs": {Name: "docs"}}}
	client, err = preparer.PrepareOpenCodeHost(context.Background(), client, envelope)
	if err != nil {
		t.Fatal(err)
	}
	host := client.OpenCodeHost.(*Snapshot)
	if host.Profile().Family != opencodehost.V2 || host.Root() != filepath.Join(a, "missing", "opencode") || client.Version != "2.0.21" {
		t.Fatal("snapshot did not bind explicit evidence/root")
	}
	_, env := host.Target()
	env[0] = "PATH=escaped"
	profile := host.Profile()
	profile.Capabilities[opencodehost.GlobalSkillDirectory] = opencodehost.Unverified
	selections := host.Selections()
	selections[0].Adapter = opencodehost.ObserverV1
	if err := host.ValidateNative(true, nil); err != nil {
		t.Fatal(err)
	}
	if err := preparer.RevalidateOpenCodeHost(context.Background(), client); err != nil {
		t.Fatal(err)
	}
	before := calls
	if err := os.Remove(link); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(b, link); err != nil {
		t.Fatal(err)
	}
	if err := preparer.RevalidateOpenCodeHost(context.Background(), client); !errors.Is(err, ErrPlanChanged) || calls != before {
		t.Fatalf("root fence: probes=%d before=%d %v", calls, before, err)
	}
}

// RED if metadata-only or unsupported historical components require a probe,
// or missing/unverified native targets obtain fabricated default authority.
func TestSharedOpenCodeNoEffectAndUnverifiedTarget(t *testing.T) {
	root := t.TempDir()
	client := domain.DetectedClient{ClientID: domain.ClientOpenCode, ConfigRoot: root, ExecutablePath: filepath.Join(root, "synthetic99-script"), Version: "99.0.0"}
	calls := 0
	preparer, err := New(func(context.Context, clientdetect.ProbeTarget) (clientdetect.ProbeEvidence, error) {
		calls++
		return clientdetect.ProbeEvidence{}, nil
	}, []string{"PATH="})
	if err != nil {
		t.Fatal(err)
	}
	for _, env := range []domain.PackageEnvelope{{}, {MCP: domain.MCPComponent{Servers: map[string]domain.MCPServer{"historical": {Type: "sse"}}}}} {
		out, err := preparer.PrepareOpenCodeHost(context.Background(), client, env)
		if err != nil || out.OpenCodeHost != nil || calls != 0 {
			t.Fatalf("no-effect: %+v calls=%d %v", out, calls, err)
		}
	}
	desired := domain.PackageEnvelope{Skills: map[string]domain.Skill{"docs": {Name: "docs"}}}
	if out, err := preparer.PrepareOpenCodeHost(context.Background(), client, desired); err == nil || out.OpenCodeHost != nil {
		t.Fatalf("unverified target qualified: %+v %v", out, err)
	}
	client.ExecutablePath = ""
	before := calls
	if _, err := preparer.PrepareOpenCodeHost(context.Background(), client, desired); !errors.Is(err, ErrHostTargetRequired) || calls != before {
		t.Fatalf("missing target: %v", err)
	}
}
