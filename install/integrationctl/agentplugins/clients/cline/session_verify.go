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
func (*Adapter) VerifySession(ctx context.Context, env clients.Env, plan clients.SessionPlan) (resultErr error) {
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
	pin, err := authorizedSessionSourcePin(bounded, env.NativeConfig, plan.Environment["CLINE_MCP_SETTINGS_PATH"])
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, pin.Close()) }()
	return verifyPinnedSession(bounded, env, plan, pin)
}

func verifyPinnedSession(ctx context.Context, env clients.Env, plan clients.SessionPlan, pin *sessionSourcePin) error {
	before, err := sessionSourceSnapshot(ctx, env.NativeConfig, plan, pin)
	if err != nil {
		return err
	}
	// Source verification is read-only but can be slow: revoked consent must
	// never reach even a runner that launches before checking cancellation.
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("cline selected-source observation canceled before native start: %w", err)
	}
	if plan.Environment["CLINE_MCP_SETTINGS_PATH"] != pin.path {
		return nativeconfig.ErrConcurrentChange
	}
	if err := pin.Revalidate(ctx); err != nil {
		return err
	}
	result, err := env.Runner.Run(ctx, legacyports.Command{
		Argv: []string{plan.Executable, "--auto-approve", "false", "--cwd", plan.Directory, "config", "mcp", "--json"},
		Dir:  plan.Directory, Env: plan.ChildEnvironment(os.Environ()), StdoutLimitBytes: 1 << 20,
	})
	if err != nil {
		return fmt.Errorf("cline selected-source observation failed: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("cline selected-source observation expired: %w", err)
	}
	if result.ExitCode != 0 || len(result.Stdout) > 1<<20 || !result.CleanupComplete || result.NoChildStarted {
		return fmt.Errorf("cline configuration inventory failed, exceeded its bound or did not prove process cleanup")
	}
	after, err := sessionSourceSnapshot(ctx, env.NativeConfig, plan, pin)
	if err != nil || after.Mode != before.Mode || !bytes.Equal(after.Body, before.Body) {
		return fmt.Errorf("cline Project MCP source changed during native inspection: %w", errors.Join(err, nativeconfig.ErrConcurrentChange))
	}
	if err := pin.Revalidate(ctx); err != nil {
		return err
	}
	if err := verifySessionInventory(result.Stdout, plan.Environment["CLINE_MCP_SETTINGS_PATH"], plan.Servers); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("cline selected-source observation expired: %w", err)
	}
	return nil
}

// A native inventory can show the same path/server names after a foreign
// command or tool approval is introduced. Rejoin owned bytes on both sides of
// the read-only native process, not merely the inventory's projected names.
func sessionSourceSnapshot(ctx context.Context, kernel nativeconfig.Kernel, plan clients.SessionPlan, pin *sessionSourcePin) (nativeconfig.FileSnapshot, error) {
	path := plan.Environment["CLINE_MCP_SETTINGS_PATH"]
	if path == "" || path != pin.path || len(plan.ConfigDigest) != 64 {
		return nativeconfig.FileSnapshot{}, fmt.Errorf("cline selected-source receipt is incomplete")
	}
	if err := pin.Revalidate(ctx); err != nil {
		return nativeconfig.FileSnapshot{}, err
	}
	file, err := kernel.ReadExactFileBounded(path, sessionSourceLimit)
	if err != nil {
		return nativeconfig.FileSnapshot{}, err
	}
	if err := ctx.Err(); err != nil {
		return nativeconfig.FileSnapshot{}, err
	}
	if !file.Exists || fmt.Sprintf("%x", sha256.Sum256(file.Body)) != plan.ConfigDigest {
		return nativeconfig.FileSnapshot{}, fmt.Errorf("cline selected-source bytes differ from their owned plan: %w", nativeconfig.ErrConcurrentChange)
	}
	if err := exactSessionServers(file.Body, plan.Servers); err != nil {
		return nativeconfig.FileSnapshot{}, err
	}
	held, err := pin.Snapshot(ctx)
	if err != nil {
		return nativeconfig.FileSnapshot{}, err
	}
	if file.Mode != held.Mode || !bytes.Equal(file.Body, held.Body) {
		return nativeconfig.FileSnapshot{}, fmt.Errorf("cline selected-source snapshot differs from its held file: %w", nativeconfig.ErrConcurrentChange)
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
