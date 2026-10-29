package agentpluginscli

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli/management"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func TestManagementLocalSkillRejectsAliasedCLIOwnership(t *testing.T) {
	for _, fixture := range []struct {
		name, refusalCode       string
		rootExists, skillExists bool
	}{
		{"dangling-root-alias", "OWNERSHIP_UNAVAILABLE", false, false},
		{"missing-skill", "MANAGED_SKILL", true, false},
		{"existing-skill", "MANAGED_SKILL", true, true},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			b, root := localSkillTestBackend(t)
			if fixture.rootExists {
				if err := os.MkdirAll(root, 0700); err != nil {
					t.Fatal(err)
				}
			}
			alias := filepath.Join(t.TempDir(), "cli-profile-alias")
			if err := os.Symlink(root, alias); err != nil {
				if runtime.GOOS == "windows" {
					t.Skip("symbolic links unavailable:", err)
				}
				t.Fatal(err)
			}
			filename := filepath.Join(root, "draft", "SKILL.md")
			var expected *string
			if fixture.skillExists {
				localSkillWrite(t, filename, "old")
				digest := localSkillDigest([]byte("old"))
				expected = &digest
			}
			request := localSkillTestRequest(localSkillTestFile("SKILL.md", expected, []byte("new")))
			for _, owner := range []struct {
				name, relative string
				locator        bool
			}{
				{"locator-root", ".", true},
				{"locator-skill", "draft", true},
				{"native-skill", "draft", false},
				{"native-missing-leaf", filepath.Join("draft", "missing", "owned"), false},
			} {
				t.Run(owner.name, func(t *testing.T) {
					prepared := localSkillPrepare(t, b, request, strings.Repeat("a", 64))
					original := b.app.StateStore
					state, err := original.Load()
					if err != nil {
						t.Fatal(err)
					}
					binding := domain.ClientBinding{}
					ownedPath := filepath.Join(alias, owner.relative)
					if owner.locator {
						binding.TargetLocator = ownedPath
					} else {
						binding.NativeObjects = []domain.NativeObjectOwnership{{Path: ownedPath, ProtectionClass: "managed"}}
					}
					state.Installations = append(state.Installations, domain.Installation{Clients: map[string]domain.ClientBinding{"native": binding}})
					b.app.StateStore = &localSkillObservedState{state: state}
					t.Cleanup(func() { b.app.StateStore = original })
					out, err := prepared.Apply(t.Context(), nil)
					var refused *management.Refusal
					if !errors.As(err, &refused) || refused.Code != fixture.refusalCode || out.Result.Mutated {
						t.Fatal("aliased CLI ownership lost after review", out, err)
					}
					_, err = b.PrepareLocalSkill(t.Context(), request, strings.Repeat("b", 64))
					if !errors.As(err, &refused) || refused.Code != fixture.refusalCode {
						t.Fatal("aliased CLI ownership ignored during review", err)
					}
					if fixture.skillExists {
						if body := dependencyRead(t, filename); body != "old" {
							t.Fatal("managed content changed", body)
						}
					} else if _, err := os.Stat(filepath.Dir(filename)); !os.IsNotExist(err) {
						t.Fatal("managed missing path was created", err)
					}
				})
			}
			if fixture.rootExists {
				if err := localSkillCheckOwnershipPath(filepath.Join(b.config.AuthoringRoots[0].Path, "draft"), filepath.Join(alias, "draft-sibling")); err != nil {
					t.Fatal("unrelated aliased sibling blocked local authoring", err)
				}
			} else if _, err := os.Stat(root); !os.IsNotExist(err) {
				t.Fatal("dangling owned root was created", err)
			}
		})
	}
}

func TestManagementLocalSkillUnresolvableOwnershipFailsClosed(t *testing.T) {
	target := filepath.Join(t.TempDir(), "draft")
	loop := filepath.Join(t.TempDir(), "owner-loop")
	if err := os.Symlink(loop, loop); err != nil {
		if runtime.GOOS == "windows" {
			t.Skip("symbolic links unavailable:", err)
		}
		t.Fatal(err)
	}
	err := localSkillCheckOwnershipPath(target, filepath.Join(loop, "managed"))
	var refused *management.Refusal
	if !errors.As(err, &refused) || refused.Code != "OWNERSHIP_UNAVAILABLE" {
		t.Fatal("unresolvable absolute ownership was ignored", err)
	}
}

func TestManagementLocalSkillPreservesAndChecksObservedFileMode(t *testing.T) {
	b, root := localSkillTestBackend(t)
	filename := filepath.Join(root, "draft", "SKILL.md")
	localSkillWrite(t, filename, "old")
	if err := os.Chmod(filename, 0444); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(filename, 0666) })
	before, err := os.Stat(filename)
	if err != nil {
		t.Fatal(err)
	}
	old := localSkillDigest([]byte("old"))
	localSkillApply(t, localSkillPrepare(t, b, localSkillTestRequest(localSkillTestFile("SKILL.md", &old, []byte("reviewed"))), strings.Repeat("c", 64)))
	after, err := os.Stat(filename)
	if err != nil || after.Mode().Perm() != before.Mode().Perm() {
		t.Fatal("editing changed an observed read-only mode", after, err)
	}
	digest := localSkillDigest([]byte("reviewed"))
	stale := localSkillPrepare(t, b, localSkillTestRequest(localSkillTestFile("SKILL.md", &digest, []byte("unreviewed"))), strings.Repeat("d", 64))
	if err := os.Chmod(filename, 0666); err != nil {
		t.Fatal(err)
	}
	out, err := stale.Apply(t.Context(), nil)
	if !errors.Is(err, management.ErrPlanStale) || out.Result.Mutated || dependencyRead(t, filename) != "reviewed" {
		t.Fatal("unreviewed permission change was ignored", out, err)
	}
}
