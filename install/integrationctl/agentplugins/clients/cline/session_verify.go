package cline

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	legacyports "github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

// VerifySession checks the actual capability used, not an exact version pin.
// This native command lists configuration only; it starts no MCP/model and
// cannot turn the persisted Prepared receipt into Active.
func (*Adapter) VerifySession(ctx context.Context, env clients.Env, plan clients.SessionPlan) error {
	// A context canceled before admission must never reach the native runner.
	// Some runners may observe cancellation only after spawning a child.
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("cline selected-source observation canceled before start: %w", err)
	}
	if env.Runner == nil {
		return fmt.Errorf("cline native configuration observer unavailable")
	}
	bounded, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	result, err := env.Runner.Run(bounded, legacyports.Command{
		Argv: []string{plan.Executable, "--auto-approve", "false", "--cwd", plan.Directory, "config", "mcp", "--json"},
		Dir:  plan.Directory, Env: plan.ChildEnvironment(os.Environ()), StdoutLimitBytes: 1 << 20,
	})
	if err != nil {
		return fmt.Errorf("cline selected-source observation failed: %w", err)
	}
	if err := bounded.Err(); err != nil {
		return fmt.Errorf("cline selected-source observation expired: %w", err)
	}
	if result.ExitCode != 0 || len(result.Stdout) > 1<<20 || !result.CleanupComplete || result.NoChildStarted {
		return fmt.Errorf("cline configuration inventory failed, exceeded its bound or did not prove process cleanup")
	}
	return verifySessionInventory(result.Stdout, plan.Environment["CLINE_MCP_SETTINGS_PATH"], plan.Servers)
}

func verifySessionInventory(body []byte, path string, names []string) error {
	decoder := json.NewDecoder(bytes.NewReader(body))
	value, err := shared.DecodeUniqueJSONValue(decoder)
	if err != nil {
		return fmt.Errorf("cline inventory JSON: %w", err)
	}
	if _, err := decoder.Token(); err != io.EOF {
		return fmt.Errorf("cline inventory contains trailing data")
	}
	rows, ok := value.([]any)
	if !ok || len(rows) != len(names) {
		return fmt.Errorf("cline inventory differs from owned project servers")
	}
	expected := map[string]bool{}
	for _, name := range names {
		expected[name] = true
	}
	for _, item := range rows {
		row, ok := item.(map[string]any)
		if !ok {
			return fmt.Errorf("cline inventory contains an invalid member")
		}
		name, _ := row["name"].(string)
		transport, _ := row["transportType"].(string)
		if !expected[name] || row["path"] != path || row["disabled"] != false || transport != "stdio" {
			return fmt.Errorf("cline did not select the exact owned project source")
		}
		// File-level verification rejects automatic approvals. If the native
		// inventory exposes its effective approval state, enforce it too:
		// startup consent must not become tool-call consent.
		if approval, present := row["autoApprove"]; present {
			items, ok := approval.([]any)
			if !ok || len(items) != 0 {
				return fmt.Errorf("cline native inventory reports automatic MCP approvals for %q", name)
			}
		}
		delete(expected, name)
	}
	return nil
}
