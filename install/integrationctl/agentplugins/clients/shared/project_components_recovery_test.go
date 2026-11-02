package shared

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
)

func componentAssertBody(t *testing.T, path, want string) {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil || string(body) != want {
		t.Fatalf("component bytes = %q, %v; want %q", body, err, want)
	}
}

func componentRecovery(t *testing.T, mutation ProjectComponentMutation) string {
	t.Helper()
	roots, err := filepath.Glob(filepath.Join(mutation.Layout.SkillsRoot, ".agentplugins-native-*"))
	if err != nil || len(roots) != 1 {
		t.Fatalf("retained recovery roots = %v, %v", roots, err)
	}
	return roots[0]
}

func componentInstalled(t *testing.T) ProjectComponentMutation {
	t.Helper()
	mutation := componentFixture(t)
	if err := mutation.Apply(context.Background()); err != nil {
		t.Fatal(err)
	}
	return mutation
}

func TestProjectComponentsMCPFailureRestoresExactSkill(t *testing.T) {
	mutation := componentNext(t, componentInstalled(t))
	before, _ := os.ReadFile(mutation.Layout.ConfigPaths.JSON)
	mutation.Kernel = nativeconfig.NewWithLockAcquirer(func(nativeconfig.Paths, nativeconfig.Codec) (func() error, error) {
		componentAssertBody(t, filepath.Join(mutation.Layout.SkillsRoot, "docs", "SKILL.md"), "inert version two\n")
		return nil, fmt.Errorf("inert fixture lock refusal")
	})
	if err := mutation.Apply(context.Background()); err == nil {
		t.Fatal("MCP failure not surfaced")
	}
	componentAssertBody(t, filepath.Join(mutation.Layout.SkillsRoot, "docs", "SKILL.md"), "inert version one\n")
	componentAssertBody(t, mutation.Layout.ConfigPaths.JSON, string(before))
	roots, _ := filepath.Glob(filepath.Join(mutation.Layout.SkillsRoot, ".agentplugins-native-*"))
	if len(roots) != 0 {
		t.Fatalf("successful rollback retained recoveries: %v", roots)
	}
}

func TestProjectComponentsRollbackPreservesExternallyChangedInstall(t *testing.T) {
	mutation := componentNext(t, componentInstalled(t))
	mutation.Kernel = nativeconfig.NewWithLockAcquirer(func(nativeconfig.Paths, nativeconfig.Codec) (func() error, error) {
		componentWrite(t, filepath.Join(mutation.Layout.SkillsRoot, "docs", "SKILL.md"), "external changed install\n")
		return nil, fmt.Errorf("inert fixture lock refusal")
	})
	err := mutation.Apply(context.Background())
	if err == nil || !strings.Contains(err.Error(), "recovery retained") {
		t.Fatalf("lost rollback recovery: %v", err)
	}
	componentAssertBody(t, filepath.Join(mutation.Layout.SkillsRoot, "docs", "SKILL.md"), "external changed install\n")
	recovery := componentRecovery(t, mutation)
	componentAssertBody(t, filepath.Join(recovery, "old-docs", "SKILL.md"), "inert version one\n")
	var effect *NativeEffectError
	if errors.As(err, &effect) {
		t.Fatalf("uncertain rollback claimed unchanged: %v", err)
	}
}

func TestProjectComponentsCommittedCleanupRefusesChangedRecovery(t *testing.T) {
	for _, drift := range []string{"backup", "unknown"} {
		t.Run(drift, func(t *testing.T) {
			mutation := componentNext(t, componentInstalled(t))
			mutation.Kernel = nativeconfig.NewWithLockAcquirer(func(nativeconfig.Paths, nativeconfig.Codec) (func() error, error) {
				return func() error {
					root := componentRecovery(t, mutation)
					if drift == "backup" {
						componentWrite(t, filepath.Join(root, "old-docs", "SKILL.md"), "foreign recovery change\n")
					} else {
						componentWrite(t, filepath.Join(root, "foreign.txt"), "foreign recovery\n")
					}
					return nil
				}, nil
			})
			err := mutation.Apply(context.Background())
			if !nativeconfig.IsCommittedCleanup(err) {
				t.Fatalf("committed cleanup lost authoritative effect: %v", err)
			}
			if err := mutation.Layout.Verify(mutation.Desired, false, mutation.Kernel); err != nil {
				t.Fatal(err)
			}
			root := componentRecovery(t, mutation)
			if drift == "backup" {
				componentAssertBody(t, filepath.Join(root, "old-docs", "SKILL.md"), "foreign recovery change\n")
			} else {
				componentAssertBody(t, filepath.Join(root, "foreign.txt"), "foreign recovery\n")
			}
		})
	}
}

func TestProjectComponentsCancelledBeforeMutation(t *testing.T) {
	mutation := componentFixture(t)
	before, _ := os.ReadFile(mutation.Layout.ConfigPaths.JSON)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := mutation.Apply(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancel error = %v", err)
	}
	componentAssertBody(t, mutation.Layout.ConfigPaths.JSON, string(before))
	if _, err := os.Lstat(mutation.Desired[0].Path); !os.IsNotExist(err) {
		t.Fatalf("canceled request installed skill: %v", err)
	}
}
