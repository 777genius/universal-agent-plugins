package providers

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/opencode"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/opencodehost"
	legacyports "github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

type openCodeObservationRunner struct {
	calls  []string
	config []byte
}

func (r *openCodeObservationRunner) Run(_ context.Context, c legacyports.Command) (legacyports.CommandResult, error) {
	name := c.Argv[1] + " " + c.Argv[2]
	r.calls = append(r.calls, name)
	if name == "debug config" {
		return legacyports.CommandResult{Stdout: r.config}, nil
	}
	return legacyports.CommandResult{Stdout: []byte("┌  MCP Servers\n●  ✓ local connected\n└  1 server(s)\n")}, nil
}

// The CLI's real prepared wrapper must retain native observation authority,
// while still passing the adapter's exact owned-config and native MCP checks.
func TestPreparedOpenCodeWrapperRetainsNativeObservation(t *testing.T) {
	root := t.TempDir()
	active := filepath.Join(root, "managed")
	if err := os.Mkdir(active, 0700); err != nil {
		t.Fatal(err)
	}
	kernel := nativeconfig.New()
	paths := nativeconfig.Paths{JSON: filepath.Join(root, "opencode.json"), JSONC: filepath.Join(root, "opencode.jsonc")}
	receipt, err := kernel.Apply(nativeconfig.Request{Paths: paths, Codec: nativeconfig.CodecOpenCode, Name: "local", Action: nativeconfig.ActionAdd, Server: nativeconfig.Server{Type: "stdio", Command: "/TEST/probe"}})
	if err != nil {
		t.Fatal(err)
	}
	projection := opencode.OpenCodeProjection{Version: 1, ConfigPath: receipt.Path, ConfigJSON: paths.JSON, ConfigJSONC: paths.JSONC, PackageRoot: active, MCPServers: map[string]nativeconfig.Server{"local": {Type: "stdio", Command: "/TEST/probe"}}}
	body, err := json.Marshal(projection)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(active, opencode.OpenCodeProjectionFile), body, 0600); err != nil {
		t.Fatal(err)
	}
	config, err := os.ReadFile(receipt.Path)
	if err != nil {
		t.Fatal(err)
	}
	evidence := opencodehost.VersionEvidence{Version: "1.19.4", Source: "executable_version", ProbeStatus: "ok"}
	host := &preparedOpenCodeClient{NativePrepared: opencodehost.NewNativePrepared("/TEST/opencode", root, nil, evidence, opencodehost.Resolve(evidence), nil)}
	objects := []domain.NativeObjectOwnership{{ObjectID: "opencode-mcp:local", Kind: opencode.OpenCodeMCPObjectKind, LogicalName: "local", Path: receipt.Path, ManagedDigest: receipt.Digest, ProtectionClass: "managed"}}
	request := domain.ActivationRequest{VerifyOnly: true, Client: domain.DetectedClient{ClientID: domain.ClientOpenCode, ExecutablePath: "/TEST/opencode", ConfigRoot: root, OpenCodeHost: host}, Plan: domain.DeliveryPlan{ClientID: domain.ClientOpenCode, Scope: domain.ScopeUser, NativeRegistryRoot: root, OpenCodeHost: host, Components: []domain.ComponentDecision{{Kind: domain.ComponentMCPServer, Name: "local"}}}, Delivery: domain.StagedDelivery{ClientID: domain.ClientOpenCode, ActivePath: active, NativeObjects: objects}, PreviousNativeObjects: objects}
	runner := &openCodeObservationRunner{config: config}
	preview, err := opencode.New().Activate(t.Context(), clients.Env{Runner: runner, NativeConfig: kernel}, request)
	if err != nil || preview.Activation != domain.ActivationPrepared || len(runner.calls) != 1 || runner.calls[0] != "debug config" {
		t.Fatalf("wrapped preview skipped exact observation: %+v %v calls=%v", preview, err, runner.calls)
	}
	request.VerifyOnly = false
	actual, err := opencode.New().Activate(t.Context(), clients.Env{Runner: runner, NativeConfig: kernel}, request)
	if err != nil || actual.Activation != domain.ActivationActive || len(runner.calls) != 3 || runner.calls[2] != "mcp list" {
		t.Fatalf("wrapped acting observation did not initialize MCP: %+v %v calls=%v", actual, err, runner.calls)
	}
}
