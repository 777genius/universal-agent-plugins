package opencode

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	legacyports "github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

// Regression: confirmed completion of an intact setup rewrote the owned
// objects and reported a committed effect, so a repeated add claimed a change.
// Missing owned objects must still be repaired by the ordinary write.
func TestOpenCodeIntactCompletionObservesWithoutRewrite(t *testing.T) {
	for _, tc := range []struct {
		name   string
		scope  domain.InstallScope
		remove bool
		effect domain.NativeEffectState
	}{
		{name: "project", scope: domain.ScopeProject, effect: domain.NativeEffectUnchanged},
		{name: "user", scope: domain.ScopeUser, effect: domain.NativeEffectUnchanged},
		{name: "missing MCP entry", scope: domain.ScopeProject, remove: true, effect: domain.NativeEffectCommitted},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newOpenCodeMaterializationFixture(t, tc.scope)
			f.env.Runner = skillObservationRunner(func(_ context.Context, command legacyports.Command) (legacyports.CommandResult, error) {
				switch strings.Join(command.Argv[1:], " ") {
				case "debug config":
					return legacyports.CommandResult{Stdout: f.config}, nil
				case "debug skill":
					return legacyports.CommandResult{Stdout: f.skills}, nil
				case "mcp list":
					return legacyports.CommandResult{Stdout: []byte("┌  MCP Servers\n●  ✓ local connected\n└  1 server(s)\n")}, nil
				}
				t.Fatalf("unexpected native command: %v", command.Argv)
				return legacyports.CommandResult{}, nil
			})
			request := f.request
			request.VerifyOnly, request.Replacing = false, true
			stageOpenCodeRepairSources(t, request)
			if tc.remove {
				if err := os.WriteFile(request.PreviousNativeObjects[1].Path, []byte("{}\n"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			before := openCodeMaterializationFiles(t, request)
			outcome, err := New().Activate(t.Context(), f.env, request)
			if err != nil || outcome.Activation != domain.ActivationActive || outcome.NativeEffect != tc.effect {
				t.Fatalf("completion effect = %+v, %v; want %s", outcome, err, tc.effect)
			}
			if unchanged := reflect.DeepEqual(before, openCodeMaterializationFiles(t, request)); unchanged != (tc.effect == domain.NativeEffectUnchanged) {
				t.Fatalf("owned files unchanged=%v for effect %s", unchanged, tc.effect)
			}
		})
	}
}

// The ordinary write installs Skills and MCP servers from the managed package.
func stageOpenCodeRepairSources(t *testing.T, request domain.ActivationRequest) {
	t.Helper()
	if err := os.CopyFS(filepath.Join(request.Delivery.ActivePath, "skills", "probe"), os.DirFS(request.PreviousNativeObjects[0].Path)); err != nil {
		t.Fatal(err)
	}
	projection, err := ReadOpenCodeProjection(request.Delivery.ActivePath)
	if err != nil {
		t.Fatal(err)
	}
	projection.MCPServers = map[string]nativeconfig.Server{"local": {Type: "stdio", Command: "/TEST/probe"}}
	body, err := json.Marshal(projection)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(request.Delivery.ActivePath, OpenCodeProjectionFile), body, 0600); err != nil {
		t.Fatal(err)
	}
}
