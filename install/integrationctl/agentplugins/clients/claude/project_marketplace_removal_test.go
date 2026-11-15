package claude

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/filetree"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	legacyports "github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

func claudeCachedRemoval(t *testing.T) (domain.DeactivationRequest, string) {
	t.Helper()
	request := newClaudeMarketplaceRequest(t)
	request.PreviousNativeObjects = request.Delivery.NativeObjects
	cache := filepath.Join(request.Client.ConfigRoot, filepath.FromSlash(request.PreviousNativeObjects[1].SourceRelative))
	if err := filetree.CopyDir(filepath.Join(request.Plan.ActivePath, claudeProjectBundleDirectory), cache); err != nil {
		t.Fatal(err)
	}
	return claudeMarketplaceRemoval(request), cache
}

func claudeAbsentRemovalEnv(t *testing.T) clients.Env {
	t.Helper()
	return clients.Env{Runner: claudeMarketplaceRunnerFunc(func(_ context.Context, command legacyports.Command) (legacyports.CommandResult, error) {
		if !reflect.DeepEqual(command.Argv[1:], []string{"plugin", "list", "--json"}) {
			t.Fatalf("native absence unexpectedly triggered a mutation: %v", command.Argv)
		}
		return legacyports.CommandResult{Stdout: []byte("[]")}, nil
	})}
}

func TestClaudeProjectMarketplaceRemovalReplaysNativeAbsence(t *testing.T) {
	request, cache := claudeCachedRemoval(t)
	siblings := []string{
		filepath.Join(filepath.Dir(cache), "previous-version", "foreign.txt"),
		filepath.Join(request.Client.ConfigRoot, "plugins", "cache", "another-marketplace", "foreign.txt"),
		filepath.Join(request.Client.ConfigRoot, "plugins", "data", "keep.txt"),
	}
	for _, path := range siblings {
		claudeMarketplaceWrite(t, path, "unowned bytes")
	}
	env := claudeAbsentRemovalEnv(t)
	out, err := New().Deactivate(t.Context(), env, request)
	if err != nil || !out.ExternalRemovalComplete || out.NativeEffect != domain.NativeEffectCommitted {
		t.Fatalf("cache cleanup after native absence: %+v %v", out, err)
	}
	if _, err := os.Lstat(cache); !os.IsNotExist(err) {
		t.Fatalf("current owned cache survived: %v", err)
	}
	for _, path := range siblings {
		body, err := os.ReadFile(path)
		if err != nil || string(body) != "unowned bytes" {
			t.Fatalf("cache cleanup changed an unowned sibling: %s %v", path, err)
		}
	}
	out, err = New().Deactivate(t.Context(), env, request)
	if err != nil || !out.ExternalRemovalComplete || out.NativeEffect != domain.NativeEffectUnchanged {
		t.Fatalf("already-completed cache cleanup: %+v %v", out, err)
	}
}

func TestClaudeProjectMarketplaceRemovalRefusesForeignCache(t *testing.T) {
	for _, condition := range []string{"drift", "duplicate-receipt", "foreign-source", "foreign-path", "modified-receipt", "symlink", "parent-symlink", "malformed-marker", "marker-and-drift", "git-extra", "workspace-lock-extra"} {
		t.Run(condition, func(t *testing.T) {
			request, cache := claudeCachedRemoval(t)
			preserved := cache
			switch condition {
			case "drift":
				claudeMarketplaceWrite(t, filepath.Join(cache, "foreign.txt"), "foreign bytes")
			case "duplicate-receipt":
				request.NativeObjects = append(request.NativeObjects, request.NativeObjects[1])
			case "foreign-source":
				request.NativeObjects[1].SourceRelative = "plugins/cache/foreign/demo/1.0.0"
			case "foreign-path":
				request.NativeObjects[1].Path += "-foreign"
			case "modified-receipt":
				request.NativeObjects[1].UserModified = true
			case "malformed-marker":
				claudeMarketplaceWrite(t, filepath.Join(cache, claudeOrphanedMarker), "foreign bytes")
			case "marker-and-drift":
				claudeMarketplaceWrite(t, filepath.Join(cache, claudeOrphanedMarker), "1730000000000")
				claudeMarketplaceWrite(t, filepath.Join(cache, "foreign.txt"), "foreign bytes")
			case "git-extra":
				claudeMarketplaceWrite(t, filepath.Join(cache, ".git", "foreign.txt"), "foreign bytes")
			case "workspace-lock-extra":
				claudeMarketplaceWrite(t, filepath.Join(cache, ".uap-integrations.lock"), "foreign bytes")
			case "symlink", "parent-symlink":
				path := cache
				if condition == "parent-symlink" {
					path = filepath.Dir(cache)
				}
				if err := os.Rename(path, path+"-preserved"); err != nil {
					t.Fatal(err)
				}
				preserved = path + "-preserved"
				if err := os.Symlink(preserved, path); err != nil {
					t.Skipf("symlink fixture unavailable: %v", err)
				}
			}
			out, err := New().Deactivate(t.Context(), claudeAbsentRemovalEnv(t), request)
			if err == nil || out.ExternalRemovalComplete || out.NativeEffect != domain.NativeEffectUnchanged {
				t.Fatalf("foreign cache cleanup was not refused: %+v %v", out, err)
			}
			if _, err := os.Stat(preserved); err != nil {
				t.Fatalf("foreign cache was removed: %v", err)
			}
		})
	}
}

func TestClaudeProjectMarketplaceOrphanedCacheRequiresNativeAbsence(t *testing.T) {
	request, cache := claudeCachedRemoval(t)
	claudeMarketplaceWrite(t, filepath.Join(cache, claudeOrphanedMarker), "1730000000000")
	if _, err := claudeProjectRemovalCache(t.Context(), request, false); err == nil {
		t.Fatal("preexisting orphan marker was accepted before native absence")
	}
	out, err := New().Deactivate(t.Context(), claudeAbsentRemovalEnv(t), request)
	if err != nil || !out.ExternalRemovalComplete || out.NativeEffect != domain.NativeEffectCommitted {
		t.Fatalf("retry with original receipt and native orphan marker: %+v %v", out, err)
	}
	if _, err := os.Lstat(cache); !os.IsNotExist(err) {
		t.Fatalf("verified orphaned cache survived: %v", err)
	}
}

func TestClaudeProjectMarketplaceCacheCleanupRechecksAfterNativeMutation(t *testing.T) {
	request, cache := claudeCachedRemoval(t)
	verified, err := claudeProjectRemovalCache(t.Context(), request, false)
	if err != nil {
		t.Fatal(err)
	}
	foreign := filepath.Join(cache, "foreign-after-native-uninstall.txt")
	claudeMarketplaceWrite(t, filepath.Join(cache, claudeOrphanedMarker), "1730000000000")
	claudeMarketplaceWrite(t, foreign, "preserve concurrent change")
	effect := domain.NativeEffectCommitted
	if err := removeClaudeProjectCache(t.Context(), request, verified, &effect); err == nil || effect != domain.NativeEffectCommitted {
		t.Fatalf("post-uninstall drift did not preserve mutation evidence: %s %v", effect, err)
	}
	if body, err := os.ReadFile(foreign); err != nil || string(body) != "preserve concurrent change" {
		t.Fatalf("post-uninstall foreign bytes were lost: %v", err)
	}
}
