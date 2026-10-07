package opencode

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/opencodehost"
)

func registryTestHost(version string) domain.OpenCodeHostAuthority {
	evidence := opencodehost.VersionEvidence{Version: version, Source: "executable_version", ProbeStatus: "ok", ExecutableIdentity: "TEST-registry-host"}
	return opencodehost.NewNativePrepared("TEST-unused", "TEST-unused", nil, evidence, opencodehost.Resolve(evidence), nil)
}

// RED: skills-only managed ownership chose V1 and reported RegistryClear for a
// foreign V2 leaf. Missing/unqualified authority must also refuse this new MCP.
func TestOpenCodeSkillsOnlyBindingInspectsDesiredMCPWithPreparedCodec(t *testing.T) {
	for _, tc := range []struct {
		name    string
		host    domain.OpenCodeHostAuthority
		finding clients.RegistryFinding
		refused bool
	}{
		{"qualified-v2", registryTestHost("2.0.21"), clients.RegistryCollision, false},
		{"missing-host", nil, clients.RegistryIndeterminate, true},
		{"unqualified-host", registryTestHost("2.0.22"), clients.RegistryIndeterminate, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			path := filepath.Join(root, "opencode.json")
			foreign := []byte(`{"mcp":{"servers":{"docs":{"type":"remote","url":"https://foreign.test","disabled":false}}}}`)
			if err := os.WriteFile(path, foreign, 0600); err != nil {
				t.Fatal(err)
			}
			managed := &domain.ClientBinding{NativeObjects: []domain.NativeObjectOwnership{{Kind: openCodeSkillKind, LogicalName: "old-skill", Path: filepath.Join(root, "skills", "old-skill")}}}
			plan := domain.DeliveryPlan{NativeRegistryRoot: root, OpenCodeHost: tc.host, Components: []domain.ComponentDecision{{Kind: domain.ComponentMCPServer, Name: "docs", Support: domain.SupportNative}}}
			finding, err := InspectOpenCodeRegistry(plan, managed, nativeconfig.New())
			if finding != tc.finding || (err != nil) != tc.refused {
				t.Fatalf("desired registry = %v, %v; want %v refused=%v", finding, err, tc.finding, tc.refused)
			}
			if body, err := os.ReadFile(path); err != nil || string(body) != string(foreign) {
				t.Fatalf("registry inspection changed foreign config: %s %v", body, err)
			}
		})
	}
}

// Stored receipts remain executable-independent authority when observing owned
// leaves, even if a prepared host now selects the opposite config dialect.
func TestOpenCodeStoredMCPCodecPrecedesPreparedHost(t *testing.T) {
	for _, codec := range []nativeconfig.Codec{nativeconfig.CodecOpenCode, nativeconfig.CodecOpenCodeV2} {
		t.Run(string(codec), func(t *testing.T) {
			root := t.TempDir()
			paths := nativeconfig.Paths{JSON: filepath.Join(root, "opencode.json"), JSONC: filepath.Join(root, "opencode.jsonc")}
			kernel := nativeconfig.New()
			receipt, err := kernel.Apply(nativeconfig.Request{Paths: paths, Codec: codec, Action: nativeconfig.ActionAdd, Name: "docs", Server: nativeconfig.Server{Type: "remote", URL: "https://owned.test"}})
			if err != nil {
				t.Fatal(err)
			}
			object := domain.NativeObjectOwnership{ObjectID: "opencode-mcp:docs", Kind: openCodeMCPKind(codec), LogicalName: "docs", Path: receipt.Path, ManagedDigest: receipt.Digest}
			managed := &domain.ClientBinding{NativeObjects: []domain.NativeObjectOwnership{object}}
			version := "2.0.21"
			if codec == nativeconfig.CodecOpenCodeV2 {
				version = "1.18.34"
			}
			for _, host := range []domain.OpenCodeHostAuthority{nil, registryTestHost(version)} {
				plan := domain.DeliveryPlan{NativeRegistryRoot: root, OpenCodeHost: host, Components: []domain.ComponentDecision{{Kind: domain.ComponentMCPServer, Name: "docs", Support: domain.SupportNative}}}
				finding, err := InspectOpenCodeRegistry(plan, managed, kernel)
				if finding != clients.RegistryExpected || err != nil {
					t.Fatalf("stored codec lost authority: %v %v", finding, err)
				}
			}
		})
	}
}

func TestOpenCodeUnknownAndMixedStoredCodecsRefusePreparedHost(t *testing.T) {
	for _, kinds := range [][]string{{"opencode_unknown"}, {OpenCodeMCPObjectKind, OpenCodeV2MCPObjectKind}} {
		managed := &domain.ClientBinding{}
		for _, kind := range kinds {
			managed.NativeObjects = append(managed.NativeObjects, domain.NativeObjectOwnership{Kind: kind})
		}
		plan := domain.DeliveryPlan{NativeRegistryRoot: t.TempDir(), OpenCodeHost: registryTestHost("2.0.21"), Components: []domain.ComponentDecision{{Kind: domain.ComponentMCPServer, Name: "docs", Support: domain.SupportNative}}}
		finding, err := InspectOpenCodeRegistry(plan, managed, nativeconfig.New())
		if finding != clients.RegistryIndeterminate || err == nil {
			t.Fatalf("stored kinds %v accepted: %v %v", kinds, finding, err)
		}
	}
}
