package cline

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
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
	before, err := sessionSourceSnapshot(env.NativeConfig, plan)
	if err != nil {
		return err
	}
	bounded, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	// Source verification is read-only but can be slow: revoked consent must
	// never reach even a runner that launches before checking cancellation.
	if err := bounded.Err(); err != nil {
		return fmt.Errorf("cline selected-source observation canceled before native start: %w", err)
	}
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
	after, err := sessionSourceSnapshot(env.NativeConfig, plan)
	if err != nil || after.Mode != before.Mode || !bytes.Equal(after.Body, before.Body) {
		return fmt.Errorf("cline Project MCP source changed during native inspection: %w", errors.Join(err, nativeconfig.ErrConcurrentChange))
	}
	if err := verifySessionInventory(result.Stdout, plan.Environment["CLINE_MCP_SETTINGS_PATH"], plan.Servers); err != nil {
		return err
	}
	if err := bounded.Err(); err != nil {
		return fmt.Errorf("cline selected-source observation expired: %w", err)
	}
	return nil
}

// A native inventory can show the same path/server names after a foreign
// command or tool approval is introduced. Rejoin owned bytes on both sides of
// the read-only native process, not merely the inventory's projected names.
func sessionSourceSnapshot(kernel nativeconfig.Kernel, plan clients.SessionPlan) (nativeconfig.FileSnapshot, error) {
	path := plan.Environment["CLINE_MCP_SETTINGS_PATH"]
	if path == "" || len(plan.ConfigDigest) != 64 {
		return nativeconfig.FileSnapshot{}, fmt.Errorf("cline selected-source receipt is incomplete")
	}
	file, err := kernel.ReadExactFile(path)
	if err != nil {
		return nativeconfig.FileSnapshot{}, err
	}
	if !file.Exists || len(file.Body) > 1<<20 || fmt.Sprintf("%x", sha256.Sum256(file.Body)) != plan.ConfigDigest {
		return nativeconfig.FileSnapshot{}, fmt.Errorf("cline selected-source bytes differ from their owned plan")
	}
	if err := exactSessionServers(file.Body, plan.Servers); err != nil {
		return nativeconfig.FileSnapshot{}, err
	}
	return file, nil
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
