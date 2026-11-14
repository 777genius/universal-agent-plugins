package kiro

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/atomicfile"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func kiroProjectEffectRequest(t *testing.T) domain.ActivationRequest {
	t.Helper()
	base := t.TempDir()
	project, user := filepath.Join(base, "project"), filepath.Join(base, "user", ".kiro")
	root := filepath.Join(project, ".kiro")
	active, objects := kiroNativeFixture(t, root, "new", "https://managed.test")
	target := filepath.Join(project, ".uap", "managed", "clients", "kiro", "demo-0123456789ab")
	if err := os.MkdirAll(filepath.Dir(target), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(active, target); err != nil {
		t.Fatal(err)
	}
	request := kiroActivationRequest(root, target, nil, objects)
	request.Client.ConfigRoot = user
	request.Plan.Scope = domain.ScopeProject
	request.Plan.NativeRegistryRoot = root
	request.Plan.ActivePath = target
	request.Plan.PhysicalArtifactID = "demo-0123456789ab"
	var err error
	request.Plan.TargetContext, err = domain.NewTargetContext(domain.ScopeProject, project, user)
	if err != nil {
		t.Fatal(err)
	}
	return request
}

func TestProjectKiroVisibleNativeWriteFailureRollsBackAndKeepsOwnership(t *testing.T) {
	request := kiroProjectEffectRequest(t)
	path := filepath.Join(request.Plan.NativeRegistryRoot, "settings", "mcp.json")
	original := []byte(`{"mcpServers":{"foreign":{"url":"https://foreign.test","disabled":true}}}`)
	writeTestFile(t, path, string(original))
	writeErr := errors.New("inert visible write failure")
	files := &kiroFaultFiles{write: func(path string, body []byte, mode os.FileMode, count int) error {
		if err := atomicfile.Write(path, body, mode); err != nil {
			return err
		}
		if count == 1 {
			return writeErr
		}
		return nil
	}}
	outcome, err := New().Activate(context.Background(), clients.Env{NativeConfig: nativeconfig.NewWithFileIO(files)}, request)
	if !errors.Is(err, writeErr) || outcome.NativeEffect != domain.NativeEffectUnchanged || len(outcome.NativeObjects) != 0 {
		t.Fatalf("project rollback=%+v,%v", outcome, err)
	}
	body, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(body, original) || files.writes != 2 {
		t.Fatalf("project rollback bytes=%q,%v writes=%d", body, err, files.writes)
	}
	if _, err := os.Stat(filepath.Join(request.Plan.NativeRegistryRoot, "skills", "docs")); !os.IsNotExist(err) {
		t.Fatalf("project skill survived failed MCP write: %v", err)
	}
	if _, err := os.Stat(request.Client.ConfigRoot); !os.IsNotExist(err) {
		t.Fatalf("project rollback touched global config: %v", err)
	}
}

func TestProjectKiroCommittedCleanupRetainsPreparedReceipts(t *testing.T) {
	request := kiroProjectEffectRequest(t)
	releaseErr := errors.New("inert lock cleanup failure")
	kernel := nativeconfig.NewWithLockAcquirer(func(nativeconfig.Paths, nativeconfig.Codec) (func() error, error) {
		return func() error { return releaseErr }, nil
	})
	outcome, err := New().Activate(context.Background(), clients.Env{NativeConfig: kernel}, request)
	if err != nil || outcome.NativeEffect != domain.NativeEffectCommitted || outcome.Activation != domain.ActivationPrepared || outcome.Verification != domain.VerificationInstalled || len(outcome.NativeObjects) != 2 || !strings.Contains(strings.Join(outcome.UserActions, " "), "cleanup degraded") {
		t.Fatalf("project committed cleanup=%+v,%v", outcome, err)
	}
	if err := VerifyNativeObjects(request.Plan.NativeRegistryRoot, outcome.NativeObjects, false); err != nil {
		t.Fatal(err)
	}
}
