package agentpluginscli

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli/management"
)

func TestManagementLocalSkillRestoreChecksRetainedReceipt(t *testing.T) {
	for _, name := range []string{
		"exact-size-limit", "over-size-limit", "trailing-document", "directory",
		"linked-file-inside", "linked-parent-inside", "linked-file-outside",
	} {
		t.Run(name, func(t *testing.T) {
			b, root := localSkillTestBackend(t)
			active := filepath.Join(root, "draft", "SKILL.md")
			localSkillWrite(t, active, "original")
			before := localSkillDigest([]byte("original"))
			id := strings.Repeat("1", 64)
			out := localSkillApply(t, localSkillPrepare(t, b, localSkillTestRequest(localSkillTestFile("SKILL.md", &before, []byte("reviewed"))), id))
			if out.Result.LocalSkillBackup == nil {
				t.Fatal("edit did not retain a backup")
			}
			backup := filepath.Join(out.Result.LocalSkillBackup.Path, "SKILL.md")
			filename := b.localSkillReceiptPath(id)
			body := dependencyRead(t, filename)
			switch name {
			case "exact-size-limit", "over-size-limit":
				size := 65536
				if name == "over-size-limit" {
					size++
				}
				if len(body) >= size {
					t.Fatal("fixture receipt already reaches its size limit")
				}
				dependencyWrite(t, filename, body+strings.Repeat(" ", size-len(body)))
			case "trailing-document":
				dependencyWrite(t, filename, body+"{}")
			case "directory":
				if err := os.Remove(filename); err != nil {
					t.Fatal(err)
				}
				if err := os.Mkdir(filename, 0700); err != nil {
					t.Fatal(err)
				}
			case "linked-file-inside", "linked-parent-inside", "linked-file-outside":
				original := filename
				if name == "linked-parent-inside" {
					original = filepath.Dir(filename)
				}
				retained := original + "-retained"
				if name == "linked-file-outside" {
					retained = filepath.Join(t.TempDir(), "receipt.json")
				}
				if err := os.Rename(original, retained); err != nil {
					t.Fatal(err)
				}
				target, err := filepath.Rel(filepath.Dir(original), retained)
				if err != nil {
					t.Fatal(err)
				}
				if err = os.Symlink(target, original); err != nil {
					if runtime.GOOS == "windows" {
						t.Skip("symbolic links unavailable:", err)
					}
					t.Fatal(err)
				}
			}
			prepared, err := b.PrepareLocalSkill(t.Context(), management.LocalSkillRequest{
				Action: management.LocalSkillRequestAction("restore"), RootId: "project-local",
				SkillName: "draft", RestoreOperationId: id,
			}, strings.Repeat("2", 64))
			if prepared != nil {
				t.Cleanup(prepared.Close)
			}
			if name == "exact-size-limit" {
				if err != nil {
					t.Fatal("valid receipt at the size limit cannot be restored:", err)
				}
				localSkillApply(t, prepared)
				if got := dependencyRead(t, active); got != "original" {
					t.Fatal("valid receipt did not restore the original Skill:", got)
				}
				return
			}
			if err == nil || prepared != nil {
				t.Fatal("unsafe retained receipt was accepted for restoration", prepared, err)
			}
			if got := dependencyRead(t, active); got != "reviewed" {
				t.Fatal("refused restoration changed the active Skill:", got)
			}
			if got := dependencyRead(t, backup); got != "original" {
				t.Fatal("refused restoration changed the retained backup:", got)
			}
		})
	}
}
