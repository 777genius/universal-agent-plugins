package providers_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/atomicfile"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

type devinFailAfterVisibleWrite struct{ fail bool }

func (*devinFailAfterVisibleWrite) ReadNoFollow(path string) ([]byte, os.FileMode, bool, error) {
	snapshot, err := nativeconfig.New().ReadExactFile(path)
	return snapshot.Body, snapshot.Mode, snapshot.Exists, err
}
func (files *devinFailAfterVisibleWrite) WriteAtomic(path string, body []byte, mode os.FileMode) error {
	if err := atomicfile.Write(path, body, mode); err != nil {
		return err
	}
	if files.fail {
		files.fail = false
		return errors.New("inert failure after visible Devin MCP replacement")
	}
	return nil
}
func (*devinFailAfterVisibleWrite) RemoveNoFollow(path string) error { return os.Remove(path) }

func TestProjectDevinVisibleMCPWriteFailureRestoresNativeSkillAndBytes(t *testing.T) {
	f := newDevinProjectFixture(t, false)
	out, err := f.activator(nativeconfig.New()).Activate(context.Background(), f.request)
	devinAssertPrepared(t, out, err)
	root := f.request.Plan.NativeRegistryRoot
	before, user, other := projectGeminiInventory(t, root), projectGeminiInventory(t, f.user), projectGeminiInventory(t, f.other)
	f.request.PreviousNativeObjects = out.NativeObjects
	projectGeminiWrite(t, filepath.Join(f.source, "skills", "docs", "SKILL.md"), "---\nname: docs\ndescription: Inert new skill\n---\nNew bytes.\n")
	projectGeminiWrite(t, filepath.Join(f.source, "mcp.json"), `{"$schema":"https://agent-plugins.org/schemas/1.0.0/mcp.schema.json","mcpServers":{"docs":{"type":"streamable-http","url":"https://managed.test/new"}}}`)
	f.stage(t, "fault-update")
	kernel := nativeconfig.NewWithFileIO(&devinFailAfterVisibleWrite{fail: true})
	failed, err := f.activator(kernel).Activate(context.Background(), f.request)
	if err == nil || failed.NativeEffect == domain.NativeEffectCommitted {
		t.Fatalf("visible failure misreported: %+v,%v", failed, err)
	}
	if !reflect.DeepEqual(before, projectGeminiInventory(t, root)) || !reflect.DeepEqual(user, projectGeminiInventory(t, f.user)) || !reflect.DeepEqual(other, projectGeminiInventory(t, f.other)) {
		t.Fatal("visible write failure failed exact rollback or affected another scope")
	}
}

func TestProjectDevinCommittedCleanupKeepsNewNativeReceipts(t *testing.T) {
	f := newDevinProjectFixture(t, false)
	kernel := nativeconfig.NewWithLockAcquirer(func(nativeconfig.Paths, nativeconfig.Codec) (func() error, error) {
		return func() error { return errors.New("inert committed unlock failure") }, nil
	})
	out, err := f.activator(kernel).Activate(context.Background(), f.request)
	devinAssertPrepared(t, out, err)
	if out.NativeEffect != domain.NativeEffectCommitted || !reflect.DeepEqual(out.NativeObjects, f.request.Delivery.NativeObjects) || !strings.Contains(strings.Join(out.UserActions, " "), "lock cleanup degraded") {
		t.Fatalf("committed cleanup lost authority: %+v", out)
	}
	f.request.PreviousNativeObjects = out.NativeObjects
	f.request.VerifyOnly = true
	verified, err := f.activator(nativeconfig.New()).Activate(context.Background(), f.request)
	devinAssertPrepared(t, verified, err)
	if !reflect.DeepEqual(out.NativeObjects, verified.NativeObjects) {
		t.Fatal("committed cleanup receipts were not reusable")
	}
}
