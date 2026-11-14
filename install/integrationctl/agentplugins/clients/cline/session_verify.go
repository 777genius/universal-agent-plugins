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
	"path/filepath"
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
	// Retain the original file and parent directory while the native client
	// inspects configuration. Matching bytes alone cannot detect replacement.
	pin, err := pinSessionSource(plan.Environment["CLINE_MCP_SETTINGS_PATH"])
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, pin.Close()) }()
	before, err := sessionSourceSnapshot(env.NativeConfig, plan)
	if err != nil {
		return err
	}
	if err := pin.Revalidate(); err != nil {
		return err
	}
	bounded, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	// Source verification is read-only but can be slow: revoked consent must
	// never reach even a runner that launches before checking cancellation.
	if err := bounded.Err(); err != nil {
		return fmt.Errorf("cline selected-source observation canceled before native start: %w", err)
	}
	if err := pin.Revalidate(); err != nil {
		return err
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
	if err := pin.Revalidate(); err != nil {
		return err
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

// sessionSourcePin is a read-only physical lease for the selected MCP source.
// Both handles stay open until observation finishes. The native client still
// reads the documented pathname; we cannot atomically fence noncooperating
// writers between the final revalidation and their own open syscall.
type sessionSourcePin struct {
	parentPath string
	filePath   string
	resolvedParent string
	parent, file *os.File
	parentInfo, fileInfo os.FileInfo
}

func pinSessionSource(path string) (selected *sessionSourcePin, resultErr error) {
	if path == "" || !filepath.IsAbs(path) || filepath.Clean(path) != path {
		return nil, fmt.Errorf("cline selected MCP source path must be absolute and clean")
	}
	parentPath := filepath.Dir(path)
	originalParent, err := os.Lstat(parentPath)
	if err != nil || !originalParent.IsDir() || originalParent.Mode()&os.ModeSymlink != 0 {
		return nil, fmt.Errorf("cline selected MCP parent is not an ordinary directory: %w", errors.Join(err, nativeconfig.ErrConcurrentChange))
	}
	resolved, err := filepath.EvalSymlinks(parentPath)
	if err != nil {
		return nil, err
	}
	parent, err := os.Open(parentPath)
	if err != nil {
		return nil, err
	}
	defer func() {
		if resultErr != nil {
			resultErr = errors.Join(resultErr, parent.Close())
		}
	}()
	parentInfo, err := parent.Stat()
	if err != nil || !sameSessionPhysicalIdentity(originalParent, parentInfo, true) {
		return nil, errors.Join(err, nativeconfig.ErrConcurrentChange)
	}
	originalFile, err := os.Lstat(path)
	if err != nil || !originalFile.Mode().IsRegular() || originalFile.Size() > 1<<20 {
		return nil, fmt.Errorf("cline selected MCP file is missing, aliased or oversized: %w", errors.Join(err, nativeconfig.ErrConcurrentChange))
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() {
		if resultErr != nil {
			resultErr = errors.Join(resultErr, file.Close())
		}
	}()
	fileInfo, err := file.Stat()
	if err != nil || !sameSessionPhysicalIdentity(originalFile, fileInfo, false) {
		return nil, errors.Join(err, nativeconfig.ErrConcurrentChange)
	}
	selected = &sessionSourcePin{parentPath: parentPath, filePath: path, resolvedParent: resolved,
		parent: parent, file: file, parentInfo: parentInfo, fileInfo: fileInfo}
	if err := selected.Revalidate(); err != nil {
		return nil, err
	}
	return selected, nil
}

func (pin *sessionSourcePin) Revalidate() error {
	parent, err := os.Lstat(pin.parentPath)
	if err != nil || !sameSessionPhysicalIdentity(pin.parentInfo, parent, true) {
		return fmt.Errorf("cline selected MCP parent identity changed: %w", errors.Join(err, nativeconfig.ErrConcurrentChange))
	}
	parentHeld, err := pin.parent.Stat()
	if err != nil || !sameSessionPhysicalIdentity(pin.parentInfo, parentHeld, true) {
		return fmt.Errorf("cline held MCP parent identity changed: %w", errors.Join(err, nativeconfig.ErrConcurrentChange))
	}
	resolved, err := filepath.EvalSymlinks(pin.parentPath)
	if err != nil || resolved != pin.resolvedParent {
		return fmt.Errorf("cline selected MCP ancestry changed: %w", errors.Join(err, nativeconfig.ErrConcurrentChange))
	}
	named, err := os.Lstat(pin.filePath)
	if err != nil || !sameSessionPhysicalIdentity(pin.fileInfo, named, false) {
		return fmt.Errorf("cline selected MCP file identity changed: %w", errors.Join(err, nativeconfig.ErrConcurrentChange))
	}
	held, err := pin.file.Stat()
	if err != nil || !sameSessionPhysicalIdentity(pin.fileInfo, held, false) {
		return fmt.Errorf("cline held MCP file identity changed: %w", errors.Join(err, nativeconfig.ErrConcurrentChange))
	}
	return nil
}

// Physical identity is mandatory even when a replacement retains identical
// bytes, permission bits, length and timestamps. A no-follow named observation
// is joined against the original held object, not treated as new authority.
func sameSessionPhysicalIdentity(original, observed os.FileInfo, directory bool) bool {
	if original == nil || observed == nil || !os.SameFile(original, observed) ||
		original.Mode() != observed.Mode() || original.Size() != observed.Size() ||
		!original.ModTime().Equal(observed.ModTime()) {
		return false
	}
	if directory {
		return observed.IsDir() && observed.Mode()&os.ModeSymlink == 0
	}
	return observed.Mode().IsRegular()
}

func (pin *sessionSourcePin) Close() error {
	return errors.Join(pin.file.Close(), pin.parent.Close())
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
