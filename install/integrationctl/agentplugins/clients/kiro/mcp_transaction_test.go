package kiro

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/atomicfile"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// Faults wrap real files in fresh test roots; assertions inspect those files,
// transaction recovery directories and adapter receipts after each boundary.
type kiroFaultFiles struct {
	reads, writes int
	beforeRead    func(string, int) error
	write         func(string, []byte, os.FileMode, int) error
}

func (files *kiroFaultFiles) ReadNoFollow(path string) ([]byte, os.FileMode, bool, error) {
	files.reads++
	if files.beforeRead != nil {
		if err := files.beforeRead(path, files.reads); err != nil {
			return nil, 0, false, err
		}
	}
	snapshot, err := nativeconfig.New().ReadExactFile(path)
	return snapshot.Body, snapshot.Mode, snapshot.Exists, err
}
func (files *kiroFaultFiles) WriteAtomic(path string, body []byte, mode os.FileMode) error {
	files.writes++
	if files.write != nil {
		return files.write(path, body, mode, files.writes)
	}
	return atomicfile.Write(path, body, mode)
}
func (files *kiroFaultFiles) RemoveNoFollow(path string) error { return os.Remove(path) }

func kiroActivationRequest(root, active string, previous, desired []domain.NativeObjectOwnership) domain.ActivationRequest {
	return domain.ActivationRequest{
		Client: domain.DetectedClient{ClientID: domain.ClientKiro, ConfigRoot: root},
		Plan: domain.DeliveryPlan{ClientID: domain.ClientKiro, Scope: domain.ScopeUser, InstallIntent: domain.InstallIntentPrepare, Components: []domain.ComponentDecision{
			{Kind: domain.ComponentSkill, Name: "docs", Support: domain.SupportNative},
			{Kind: domain.ComponentMCPServer, Name: "docs", Support: domain.SupportNative},
		}},
		Delivery:              domain.StagedDelivery{ClientID: domain.ClientKiro, ActivePath: active, NativeObjects: desired},
		PreviousNativeObjects: previous, BackendExecutable: filepath.Join(active, "kiro-cli"),
	}
}

func TestKiroMCPConcurrentApplyPreservesBytesAndRestoresSkills(t *testing.T) {
	for _, existed := range []bool{false, true} {
		t.Run(map[bool]string{false: "absent", true: "present"}[existed], func(t *testing.T) {
			root := filepath.Join(t.TempDir(), ".kiro")
			path := filepath.Join(root, "settings", "mcp.json")
			if existed {
				writeTestFile(t, path, `{"mcpServers":{},"original":true}`)
			}
			active, desired := kiroNativeFixture(t, root, "new", "https://new.test")
			foreign := []byte(`{"mcpServers":{"foreign":{"url":"https://foreign.test"}}}`)
			files := &kiroFaultFiles{beforeRead: func(path string, reads int) error {
				if reads == 2 {
					if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
						return err
					}
					return os.WriteFile(path, foreign, 0600)
				}
				return nil
			}}
			outcome, err := (&Adapter{}).Activate(context.Background(), clients.Env{NativeConfig: nativeconfig.NewWithFileIO(files)}, kiroActivationRequest(root, active, nil, desired))
			if !errors.Is(err, nativeconfig.ErrConcurrentChange) || outcome.NativeEffect != domain.NativeEffectUncertain || len(outcome.NativeObjects) != 0 {
				t.Fatalf("concurrent apply: %+v %v", outcome, err)
			}
			got, readErr := os.ReadFile(path)
			if readErr != nil || !bytes.Equal(got, foreign) || files.writes != 0 {
				t.Fatalf("concurrent file changed: %s %v writes=%d", got, readErr, files.writes)
			}
			if _, err := os.Lstat(desired[0].Path); !os.IsNotExist(err) {
				t.Fatalf("new skill not rolled back: %v", err)
			}
		})
	}
}

func TestKiroMCPVisibleWriteFailureRestoresExactPriorState(t *testing.T) {
	for _, existing := range []bool{false, true} {
		t.Run(map[bool]string{false: "add", true: "update"}[existing], func(t *testing.T) {
			root := filepath.Join(t.TempDir(), ".kiro")
			path := filepath.Join(root, "settings", "mcp.json")
			var previous []domain.NativeObjectOwnership
			var before []byte
			if existing {
				old, objects := kiroNativeFixture(t, root, "old", "https://old.test")
				if err := applyKiroNativeMutation(root, old, nil, objects); err != nil {
					t.Fatal(err)
				}
				previous = objects
				before, _ = os.ReadFile(path)
			}
			active, desired := kiroNativeFixture(t, root, "new", "https://new.test")
			writeErr := errors.New("visible write failure")
			files := &kiroFaultFiles{write: func(path string, body []byte, mode os.FileMode, count int) error {
				if err := atomicfile.Write(path, body, mode); err != nil {
					return err
				}
				if count == 1 {
					return writeErr
				}
				return nil
			}}
			outcome, err := (&Adapter{}).Activate(context.Background(), clients.Env{NativeConfig: nativeconfig.NewWithFileIO(files)}, kiroActivationRequest(root, active, previous, desired))
			if !errors.Is(err, writeErr) || outcome.NativeEffect != domain.NativeEffectUnchanged || !reflect.DeepEqual(outcome.NativeObjects, previous) {
				t.Fatalf("failed write: %+v %v", outcome, err)
			}
			after, readErr := os.ReadFile(path)
			if existing {
				if readErr != nil || !bytes.Equal(after, before) {
					t.Fatalf("exact original not restored: %s %v", after, readErr)
				}
				if err := VerifyNativeObjects(root, previous, false); err != nil {
					t.Fatal(err)
				}
			} else {
				if !os.IsNotExist(readErr) {
					t.Fatalf("new MCP file retained: %s %v", after, readErr)
				}
				if _, err := os.Stat(desired[0].Path); !os.IsNotExist(err) {
					t.Fatalf("new skill retained: %v", err)
				}
			}
			if files.reads < 4 {
				t.Fatalf("visible error lacked readback: %d reads", files.reads)
			}
			recovery, err := filepath.Glob(filepath.Join(root, "skills", ".agentplugins-native-*"))
			if err != nil || len(recovery) != 0 {
				t.Fatalf("successful rollback left recovery: %v %v", recovery, err)
			}
		})
	}
}

func TestKiroInstalledSkillFailedRestoreAttemptedOnceAndRetained(t *testing.T) {
	root := filepath.Join(t.TempDir(), ".kiro")
	active, previous := kiroNativeFixture(t, root, "old", "https://old.test")
	if err := applyKiroNativeMutation(root, active, nil, previous); err != nil {
		t.Fatal(err)
	}
	active, desired := kiroNativeFixture(t, root, "new", "https://new.test")
	writeErr, restoreErr := errors.New("write failure"), errors.New("restore failure")
	files := &kiroFaultFiles{write: func(string, []byte, os.FileMode, int) error { return writeErr }}
	restores := 0
	var backup string
	rename := func(src, dst string) error {
		if strings.HasPrefix(filepath.Base(src), "old-") {
			restores++
			backup = src
			return restoreErr
		}
		return shared.RenameDirectoryExclusive(src, dst)
	}
	effect, err := applyKiroNativeMutationWithKernelAndOps(root, active, previous, desired, nativeconfig.NewWithFileIO(files), rename, os.RemoveAll)
	if !errors.Is(err, writeErr) || !errors.Is(err, restoreErr) || effect != domain.NativeEffectUncertain || restores != 1 {
		t.Fatalf("rollback: %v, %s, %d restores", err, effect, restores)
	}
	body, readErr := os.ReadFile(filepath.Join(backup, "SKILL.md"))
	if readErr != nil || string(body) != "old\n" || !strings.Contains(err.Error(), backup) {
		t.Fatalf("lost backup %q: %s %v (%v)", backup, body, readErr, err)
	}
}

func TestKiroMCPConcurrentRollbackPreservesForeignFileAndReportsReceipts(t *testing.T) {
	root := filepath.Join(t.TempDir(), ".kiro")
	active, previous := kiroNativeFixture(t, root, "old", "https://old.test")
	if err := applyKiroNativeMutation(root, active, nil, previous); err != nil {
		t.Fatal(err)
	}
	active, desired := kiroNativeFixture(t, root, "new", "https://new.test")
	writeErr := errors.New("visible write failure")
	foreign := []byte(`{"mcpServers":{"foreign":{"url":"https://foreign.test"}}}`)
	files := &kiroFaultFiles{write: func(path string, body []byte, mode os.FileMode, _ int) error {
		if err := atomicfile.Write(path, body, mode); err != nil {
			return err
		}
		return writeErr
	}, beforeRead: func(path string, reads int) error {
		// original, precondition, visible-write readback, rollback observation,
		// then the immediate restore precondition: foreign wins that last boundary.
		if reads == 5 {
			return os.WriteFile(path, foreign, 0600)
		}
		return nil
	}}
	outcome, err := (&Adapter{}).Activate(context.Background(), clients.Env{NativeConfig: nativeconfig.NewWithFileIO(files)}, kiroActivationRequest(root, active, previous, desired))
	if !errors.Is(err, writeErr) || !errors.Is(err, nativeconfig.ErrConcurrentChange) || outcome.NativeEffect != domain.NativeEffectUncertain {
		t.Fatalf("rollback: %+v %v", outcome, err)
	}
	if !reflect.DeepEqual(outcome.NativeObjects, previous[:1]) {
		t.Fatalf("receipts claimed foreign MCP: %+v", outcome.NativeObjects)
	}
	got, readErr := os.ReadFile(desired[1].Path)
	if readErr != nil || !bytes.Equal(got, foreign) || files.writes != 1 {
		t.Fatalf("foreign bytes changed: %s %v writes=%d", got, readErr, files.writes)
	}
	skill, readErr := os.ReadFile(filepath.Join(desired[0].Path, "SKILL.md"))
	if readErr != nil || string(skill) != "old\n" {
		t.Fatalf("skill restore: %s %v", skill, readErr)
	}
}

func TestKiroMCPUnreadableVisibleWriteRetainsOutputAndErrorCauses(t *testing.T) {
	root := filepath.Join(t.TempDir(), ".kiro")
	active, desired := kiroNativeFixture(t, root, "new", "https://new.test")
	// Only MCP is involved, so uncertainty must identify the retained file even
	// without a skill transaction directory.
	desired = desired[1:]
	writeErr, readErr := errors.New("visible write error"), errors.New("readback unavailable")
	files := &kiroFaultFiles{write: func(path string, body []byte, mode os.FileMode, _ int) error {
		if err := atomicfile.Write(path, body, mode); err != nil {
			return err
		}
		return writeErr
	}, beforeRead: func(_ string, count int) error {
		if count >= 3 {
			return readErr
		}
		return nil
	}}
	request := kiroActivationRequest(root, active, nil, desired)
	outcome, err := (&Adapter{}).Activate(context.Background(), clients.Env{NativeConfig: nativeconfig.NewWithFileIO(files)}, request)
	if !errors.Is(err, writeErr) || !errors.Is(err, readErr) || outcome.NativeEffect != domain.NativeEffectUncertain || !strings.Contains(err.Error(), desired[0].Path) {
		t.Fatalf("readback failure: %+v %v", outcome, err)
	}
	if files.writes != 1 {
		t.Fatalf("blind rollback attempted: %d writes", files.writes)
	}
	if err := VerifyNativeObjects(root, desired, false); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(outcome.NativeObjects, desired) {
		t.Fatalf("observable exact receipts missing: %+v", outcome.NativeObjects)
	}
}

func TestKiroPreflightFailureSeedsUnchangedEffect(t *testing.T) {
	root := filepath.Join(t.TempDir(), ".kiro")
	active, desired := kiroNativeFixture(t, root, "new", "https://new.test")
	request := kiroActivationRequest(root, active, nil, desired)
	request.Plan.InstallIntent = domain.InstallIntentAutomatic
	outcome, err := (&Adapter{}).Activate(context.Background(), clients.Env{}, request)
	if err == nil || outcome.NativeEffect != domain.NativeEffectUnchanged || len(outcome.NativeObjects) != 0 {
		t.Fatalf("preflight: %+v %v", outcome, err)
	}
	if _, err := os.Lstat(root); !os.IsNotExist(err) {
		t.Fatalf("preflight mutated config: %v", err)
	}
}

func TestKiroCommittedCleanupKeepsNativeReceiptsAndBytes(t *testing.T) {
	root := filepath.Join(t.TempDir(), ".kiro")
	active, desired := kiroNativeFixture(t, root, "new", "https://new.test")
	releaseErr := errors.New("lock cleanup failed")
	kernel := nativeconfig.NewWithLockAcquirer(func(nativeconfig.Paths, nativeconfig.Codec) (func() error, error) {
		return func() error { return releaseErr }, nil
	})
	outcome, err := (&Adapter{}).Activate(context.Background(), clients.Env{NativeConfig: kernel}, kiroActivationRequest(root, active, nil, desired))
	if !errors.Is(err, releaseErr) || !nativeconfig.IsCommittedCleanup(err) || outcome.NativeEffect != domain.NativeEffectCommitted || !reflect.DeepEqual(outcome.NativeObjects, desired) {
		t.Fatalf("cleanup: %+v %v", outcome, err)
	}
	if err := VerifyNativeObjects(root, desired, false); err != nil {
		t.Fatal(err)
	}
}

func TestKiroVisibleRestoreErrorUsesObservedUnchangedEffect(t *testing.T) {
	root := filepath.Join(t.TempDir(), ".kiro")
	active, previous := kiroNativeFixture(t, root, "old", "https://old.test")
	if err := applyKiroNativeMutation(root, active, nil, previous); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(previous[1].Path)
	if err != nil {
		t.Fatal(err)
	}
	active, desired := kiroNativeFixture(t, root, "new", "https://new.test")
	writeErr, restoreErr := errors.New("visible write failure"), errors.New("visible restore failure")
	files := &kiroFaultFiles{write: func(path string, body []byte, mode os.FileMode, count int) error {
		if err := atomicfile.Write(path, body, mode); err != nil {
			return err
		}
		if count == 1 {
			return writeErr
		}
		return restoreErr
	}}
	outcome, err := (&Adapter{}).Activate(context.Background(), clients.Env{NativeConfig: nativeconfig.NewWithFileIO(files)}, kiroActivationRequest(root, active, previous, desired))
	if !errors.Is(err, writeErr) || !errors.Is(err, restoreErr) || outcome.NativeEffect != domain.NativeEffectUnchanged || !reflect.DeepEqual(outcome.NativeObjects, previous) {
		t.Fatalf("observed restore: %+v %v", outcome, err)
	}
	after, err := os.ReadFile(previous[1].Path)
	if err != nil || !bytes.Equal(after, before) || files.writes != 2 {
		t.Fatalf("restore: %s %v writes=%d", after, err, files.writes)
	}
	if err := VerifyNativeObjects(root, previous, false); err != nil {
		t.Fatal(err)
	}
	recovery, err := filepath.Glob(filepath.Join(root, "skills", ".agentplugins-native-*"))
	if err != nil || len(recovery) != 0 {
		t.Fatalf("verified restore retained recovery: %v %v", recovery, err)
	}
}
