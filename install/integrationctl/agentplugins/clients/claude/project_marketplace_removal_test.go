package claude

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
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

func TestClaudeProjectMarketplaceCacheCleanupRetainsForeignSwapAfterIdentityCheck(t *testing.T) {
	request, cache := claudeCachedRemoval(t)
	before, err := os.Lstat(cache)
	if err != nil {
		t.Fatal(err)
	}
	parent, err := os.OpenRoot(filepath.Dir(cache))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = parent.Close() }()
	if err := verifyClaudeProjectRemovalCache(t.Context(), request, cache, true); err != nil {
		t.Fatal(err)
	}
	if err := claudeProjectCacheRemovalIdentity(request.Client.ConfigRoot, cache, parent, before); err != nil {
		t.Fatal(err)
	}
	// Exact interleaving: the already-verified owned cache is renamed away
	// and a foreign same-name directory appears before retirement starts.
	original := cache + "-original-preserved"
	if err := os.Rename(cache, original); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(cache, 0700); err != nil {
		t.Fatal(err)
	}
	claudeMarketplaceWrite(t, filepath.Join(cache, "foreign.txt"), "must not be deleted")
	effect := domain.NativeEffectCommitted
	if err := retireClaudeProjectCache(t.Context(), request, cache, parent, before, &effect); err == nil || effect != domain.NativeEffectUncertain {
		t.Fatalf("post-check swapped foreign cache was accepted: %s %v", effect, err)
	}
	if _, err := os.Stat(original); err != nil {
		t.Fatalf("original owned cache was changed: %v", err)
	}
	entries, err := os.ReadDir(filepath.Dir(cache))
	if err != nil {
		t.Fatal(err)
	}
	retained := 0
	for _, entry := range entries {
		if !strings.HasPrefix(entry.Name(), ".agentplugins-cache-retire-") {
			continue
		}
		path := filepath.Join(filepath.Dir(cache), entry.Name(), "retired", "foreign.txt")
		body, err := os.ReadFile(path)
		if err != nil || string(body) != "must not be deleted" {
			t.Fatalf("foreign replacement was not retained at %q: %v", path, err)
		}
		retained++
	}
	if retained != 1 {
		t.Fatalf("expected one quarantined foreign replacement, got %d", retained)
	}
}

func TestClaudeProjectCacheRetirementRefusesReplacedDigestNamespace(t *testing.T) {
	request, cache := claudeCachedRemoval(t)
	parent, err := os.OpenRoot(filepath.Dir(cache))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = parent.Close() }()
	before, err := parent.Stat(filepath.Base(cache))
	if err != nil {
		t.Fatal(err)
	}
	skill := filepath.Join(ClaudeRuntimeDirectory, "skills", "first", "SKILL.md")
	originalBody, err := os.ReadFile(filepath.Join(cache, skill))
	if err != nil {
		t.Fatal(err)
	}
	effect := domain.NativeEffectCommitted
	var moved, replacement string
	verifiedReplacement := false
	err = retireClaudeProjectCacheWithVerifier(t.Context(), request, cache, parent, before, &effect,
		func(ctx context.Context, request domain.DeactivationRequest, original, retired string) error {
			// Held handles now reference the real original. Replace its public
			// private-directory name before the real pathname digest is read.
			name := filepath.Base(filepath.Dir(retired))
			if err := parent.Rename(name, name+"-moved"); err != nil {
				return err
			}
			moved = filepath.Join(parent.Name(), name+"-moved", "retired")
			replacement = retired
			if err := filetree.CopyDir(moved, replacement); err != nil {
				return err
			}
			claudeMarketplaceWrite(t, filepath.Join(moved, "foreign.txt"), "preserve newly added foreign bytes")
			err := verifyClaudeRetiredCache(ctx, request, original, retired)
			verifiedReplacement = err == nil
			return err
		})
	if err == nil || effect != domain.NativeEffectUncertain || !verifiedReplacement {
		t.Fatalf("path/handle replacement was not refused after real digest verification: %s %v", effect, err)
	}
	if !strings.Contains(err.Error(), "original private path") || strings.Contains(err.Error(), "retained at") {
		t.Fatalf("recovery claimed a current location after external rename: %v", err)
	}
	assertClaudeRetirementFile(t, filepath.Join(moved, skill), string(originalBody))
	assertClaudeRetirementFile(t, filepath.Join(moved, "foreign.txt"), "preserve newly added foreign bytes")
	assertClaudeRetirementFile(t, filepath.Join(replacement, skill), string(originalBody))
}

func assertClaudeRetirementFile(t *testing.T, path, expected string) {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil || string(body) != expected {
		t.Fatalf("retirement changed bytes at %s: got %q, want %q, err %v", path, body, expected, err)
	}
}
