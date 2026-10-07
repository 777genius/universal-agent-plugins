package providers

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/clientdetect"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/opencode"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/opencodehost"
)

func buildPreparedOpenCodeFixture(t *testing.T, root, version string) string {
	t.Helper()
	if err := os.MkdirAll(root, 0o700); err != nil {
		t.Fatal(err)
	}
	executable := filepath.Join(root, "opencode")
	if runtime.GOOS == "windows" {
		executable += ".exe"
	}
	cmd := exec.CommandContext(t.Context(), "go", "build", "-ldflags", "-X main.version="+version+" -X main.mode=version-only", "-o", executable, "../adapters/clientdetect/testdata/opencode_probe.go")
	if body, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("native contract fixture build: %s, %v", body, err)
	}
	return executable
}

// Real explicit-target probing must supply the profile even though the caller's
// detected-client DTO has none. PATH and later ambient changes cannot select it.
func TestOpenCodeClientPreparationFreezesExplicitNativeTarget(t *testing.T) {
	root := t.TempDir()
	v1 := buildPreparedOpenCodeFixture(t, filepath.Join(root, "v1"), "1.18.34")
	v2 := buildPreparedOpenCodeFixture(t, filepath.Join(root, "v2"), "2.0.21")
	registry, err := clients.NewRegistry(opencode.New())
	if err != nil {
		t.Fatal(err)
	}
	preparer := NewOpenCodeClientPreparation(registry)
	preparer.environment = []string{"PATH=" + filepath.Dir(v1)}
	client := domain.DetectedClient{ClientID: domain.ClientOpenCode, ExecutablePath: v2, ConfigRoot: filepath.Join(root, "config")}
	envelope := domain.PackageEnvelope{MCP: domain.MCPComponent{Servers: map[string]domain.MCPServer{"docs": {Type: "streamable-http"}}}}
	host, err := preparer.PrepareClient(t.Context(), envelope, client, nil, v2, false)
	if err != nil {
		t.Fatal(err)
	}
	codec, err := clients.DesiredOpenCodeCodec(host)
	if err != nil || codec != nativeconfig.CodecOpenCodeV2 {
		t.Fatalf("explicit V2 preparation = %v, %v", codec, err)
	}
	client.OpenCodeHost = host
	plan := domain.DeliveryPlan{OpenCodeHost: host}
	preparer.environment[0] = "PATH=/changed-after-preparation"
	if err := preparer.RevalidateClient(t.Context(), client, plan); err != nil {
		t.Fatalf("revalidation used mutable ambient environment: %v", err)
	}
	if err := host.ValidateNative(false, []string{"stdio"}); err == nil {
		t.Fatal("prepared HTTP selection authorized a new stdio effect")
	}
	file, err := os.OpenFile(v2, os.O_WRONLY|os.O_APPEND, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := file.Write([]byte("changed-after-preflight")); err != nil {
		_ = file.Close()
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := preparer.PrepareClient(t.Context(), envelope, client, nil, v2, false); !errors.Is(err, clientdetect.ErrProbeTargetChanged) {
		t.Fatalf("confirmed preparation rebaselined a changed target: %v", err)
	}
	if err := preparer.RevalidateClient(t.Context(), client, plan); !errors.Is(err, clientdetect.ErrProbeTargetChanged) {
		t.Fatalf("changed target remained authorized: %v", err)
	}
	if _, err := os.Stat(client.ConfigRoot); !os.IsNotExist(err) {
		t.Fatalf("host preparation mutated native root: %v", err)
	}
}

// Selected native effects require explicit qualified authority rather than
// inventing the old v1 codec; zero-effect packages have a separate contract.
func TestOpenCodeDesiredProjectionRequiresQualifiedHost(t *testing.T) {
	root := t.TempDir()
	registry, err := clients.NewRegistry(opencode.New())
	if err != nil {
		t.Fatal(err)
	}
	preparer := NewOpenCodeClientPreparation(registry)
	preparer.environment = []string{"PATH="}
	version := "2.0.21"
	preparer.probe = func(context.Context, clientdetect.ProbeTarget) (clientdetect.ProbeEvidence, error) {
		return clientdetect.ProbeEvidence{VersionEvidence: opencodehost.VersionEvidence{Version: version, Source: "executable_version", ProbeStatus: "ok", ExecutableIdentity: "fixture-native-bytes"}}, nil
	}
	envelope := domain.PackageEnvelope{MCP: domain.MCPComponent{Servers: map[string]domain.MCPServer{"docs": {Type: "streamable-http", Decoded: map[string]any{"url": "https://docs.test"}}}}}
	client := domain.DetectedClient{ClientID: domain.ClientOpenCode, ConfigRoot: filepath.Join(root, "native")}
	if _, err := preparer.PrepareClient(t.Context(), envelope, client, nil, "", false); err == nil {
		t.Fatal("desired projection authorized without an explicit executable")
	}
	client.ExecutablePath = filepath.Join(root, "selected-opencode")
	host, err := preparer.PrepareClient(t.Context(), envelope, client, nil, client.ExecutablePath, false)
	if err != nil {
		t.Fatal(err)
	}
	stage := filepath.Join(root, "stage")
	if err := os.Mkdir(stage, 0o700); err != nil {
		t.Fatal(err)
	}
	plan := domain.DeliveryPlan{Components: []domain.ComponentDecision{{Kind: domain.ComponentMCPServer, Name: "docs", Support: domain.SupportNative}}, OpenCodeHost: host, NativeRegistryRoot: client.ConfigRoot, ActivePath: filepath.Join(root, "managed")}
	if err := opencode.ProjectOpenCodeNative(stage, envelope, plan, ""); err != nil {
		t.Fatal(err)
	}
	projection, err := opencode.ReadOpenCodeProjection(stage)
	if err != nil || projection.Dialect != opencodehost.DialectV2 || len(projection.MCPServers) != 1 {
		t.Fatalf("qualified projection = %+v, %v", projection, err)
	}
	version = "2.0.22"
	if _, err := preparer.PrepareClient(t.Context(), envelope, client, nil, client.ExecutablePath, false); !errors.Is(err, opencodehost.ErrUnverifiedCapability) {
		t.Fatalf("desired projection admitted an unqualified host: %v", err)
	}
}

// RED if metadata-only or explicitly unsupported desired components acquire a
// codec, probe a host, choose ambiguous config, or create a native projection.
func TestOpenCodeZeroEffectPreparationNeverProbes(t *testing.T) {
	for _, unsupported := range []bool{false, true} {
		t.Run(map[bool]string{false: "metadata", true: "unsupported"}[unsupported], func(t *testing.T) {
			root := t.TempDir()
			registry, err := clients.NewRegistry(opencode.New())
			if err != nil {
				t.Fatal(err)
			}
			preparer := NewOpenCodeClientPreparation(registry)
			preparer.probe = func(context.Context, clientdetect.ProbeTarget) (clientdetect.ProbeEvidence, error) {
				t.Fatal("zero-effect preparation probed host")
				return clientdetect.ProbeEvidence{}, errors.New("forbidden")
			}
			client := domain.DetectedClient{ClientID: domain.ClientOpenCode, ConfigRoot: root}
			for _, name := range []string{"opencode.json", "opencode.jsonc"} {
				if err := os.WriteFile(filepath.Join(root, name), []byte("foreign"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			envelope := domain.PackageEnvelope{}
			plan := domain.DeliveryPlan{NativeRegistryRoot: root}
			if unsupported {
				envelope.MCP.Servers = map[string]domain.MCPServer{"docs": {Type: "sse"}}
				plan.Components = []domain.ComponentDecision{{Kind: domain.ComponentMCPServer, Name: "docs", Support: domain.SupportUnsupported}}
			}
			for _, inert := range []bool{false, true} {
				host, err := preparer.PrepareClient(t.Context(), envelope, client, nil, "", inert)
				if err != nil || host != nil {
					t.Fatalf("zero effects acquired authority: %v %v", host, err)
				}
			}
			if err := opencode.ProjectOpenCodeNative(root, envelope, plan, ""); err != nil {
				t.Fatal(err)
			}
			if _, err := os.Stat(filepath.Join(root, opencode.OpenCodeProjectionFile)); !os.IsNotExist(err) {
				t.Fatalf("zero effects wrote projection: %v", err)
			}
			for _, name := range []string{"opencode.json", "opencode.jsonc"} {
				body, err := os.ReadFile(filepath.Join(root, name))
				if err != nil || string(body) != "foreign" {
					t.Fatalf("zero effects changed config: %s %v", body, err)
				}
			}
		})
	}
}

// The selected root can change while the native --version probe is running.
// A successful version result must not authorize the newly targeted profile.
func TestOpenCodeRevalidationRejectsRootRetargetDuringProbe(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink fixture requires Unix permissions")
	}
	root := t.TempDir()
	first, second := filepath.Join(root, "first"), filepath.Join(root, "second")
	for _, dir := range []string{first, second} {
		if err := os.Mkdir(dir, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	link := filepath.Join(root, "selected")
	if err := os.Symlink(first, link); err != nil {
		t.Fatal(err)
	}
	registry, err := clients.NewRegistry(opencode.New())
	if err != nil {
		t.Fatal(err)
	}
	preparer := NewOpenCodeClientPreparation(registry)
	evidence := clientdetect.ProbeEvidence{VersionEvidence: opencodehost.VersionEvidence{Version: "1.18.34", Source: "executable_version", ProbeStatus: "ok", ExecutableIdentity: "fixture"}}
	preparer.probe = func(context.Context, clientdetect.ProbeTarget) (clientdetect.ProbeEvidence, error) {
		return evidence, nil
	}
	client := domain.DetectedClient{ClientID: domain.ClientOpenCode, ConfigRoot: link, ExecutablePath: filepath.Join(root, "native")}
	envelope := domain.PackageEnvelope{MCP: domain.MCPComponent{Servers: map[string]domain.MCPServer{"docs": {Type: "streamable-http"}}}}
	host, err := preparer.PrepareClient(t.Context(), envelope, client, nil, "", false)
	if err != nil {
		t.Fatal(err)
	}
	client.OpenCodeHost = host
	preparer.probe = func(context.Context, clientdetect.ProbeTarget) (clientdetect.ProbeEvidence, error) {
		if err := os.Remove(link); err != nil {
			return evidence, err
		}
		if err := os.Symlink(second, link); err != nil {
			return evidence, err
		}
		return evidence, nil
	}
	if err := preparer.RevalidateClient(t.Context(), client, domain.DeliveryPlan{OpenCodeHost: host}); !errors.Is(err, clientdetect.ErrProbeTargetChanged) {
		t.Fatalf("retargeted root accepted: %v", err)
	}
	entries, err := os.ReadDir(second)
	if err != nil || len(entries) != 0 {
		t.Fatalf("retargeted profile mutated: %v, %v", entries, err)
	}
}

// Empty desired declarations must not erase old cleanup effects or bypass the
// closed receipt-kind and codec fences. No host can authorize cleanup offline.
func TestOpenCodeEmptyDesiredCleanupRetainsAuthorityAndCodecFences(t *testing.T) {
	registry, err := clients.NewRegistry(opencode.New())
	if err != nil {
		t.Fatal(err)
	}
	preparer := NewOpenCodeClientPreparation(registry)
	probes := 0
	preparer.probe = func(context.Context, clientdetect.ProbeTarget) (clientdetect.ProbeEvidence, error) {
		probes++
		return clientdetect.ProbeEvidence{VersionEvidence: opencodehost.VersionEvidence{Version: "2.0.21", Source: "executable_version", ProbeStatus: "ok", ExecutableIdentity: "fixture-native"}}, nil
	}
	client := domain.DetectedClient{ClientID: domain.ClientOpenCode, ConfigRoot: t.TempDir()}
	for _, kind := range []string{nativeconfig.OpenCodeMCPObjectKind, "opencode_global_skill_directory"} {
		previous := []domain.NativeObjectOwnership{{Kind: kind}}
		if _, err := preparer.PrepareClient(t.Context(), domain.PackageEnvelope{}, client, previous, "", true); err == nil {
			t.Fatalf("offline cleanup %s lost authority fence", kind)
		}
		if _, err := preparer.PrepareClient(t.Context(), domain.PackageEnvelope{}, client, previous, "", false); err == nil {
			t.Fatalf("cleanup %s authorized without explicit executable", kind)
		}
	}
	if probes != 0 {
		t.Fatalf("cleanup probed without explicit target: %d", probes)
	}
	client.ExecutablePath = filepath.Join(t.TempDir(), "explicit-native")
	previous := []domain.NativeObjectOwnership{{Kind: nativeconfig.OpenCodeMCPObjectKind}}
	if _, err := preparer.PrepareClient(t.Context(), domain.PackageEnvelope{}, client, previous, "", false); !errors.Is(err, nativeconfig.ErrNativeMigrationRequired) {
		t.Fatalf("old V1 cleanup accepted V2 codec: %v", err)
	}
	for _, inert := range []bool{false, true} {
		if _, err := preparer.PrepareClient(t.Context(), domain.PackageEnvelope{}, client, []domain.NativeObjectOwnership{{Kind: "opencode_unknown_receipt"}}, "", inert); err == nil {
			t.Fatal("unknown owned receipt disappeared through zero-effect shortcut")
		}
	}
}
