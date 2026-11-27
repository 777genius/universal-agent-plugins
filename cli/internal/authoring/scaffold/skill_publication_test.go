package scaffold

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestSkillCheckedPublicationRevalidatesBeforeAnotherAttempt(t *testing.T) {
	for _, kind := range []string{"unchanged", "tree", "core", "collision", "cancel"} {
		t.Run(kind, func(t *testing.T) {
			root, plan, gate := skillFixture(t)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			var peer string
			checks := 0
			ops := applyOps{write: writeTree, rename: renameExclusive}
			ops.publish = func(_ context.Context, from *os.File, old string, to *os.File, new string, check func() error) error {
				if err := check(); err != nil {
					return err
				}
				checks++
				switch kind {
				case "tree":
					peer = filepath.Join(filepath.Clean(from.Name()), old, plan.name, "SKILL.md")
				case "core":
					peer = filepath.Join(root, "plugin.json")
				case "collision":
					peer = filepath.Join(root, "skills", "foreign")
					if err := os.Mkdir(filepath.Dir(peer), 0700); err != nil {
						t.Fatal(err)
					}
				case "cancel":
					cancel()
				}
				if peer != "" {
					if err := os.WriteFile(peer, []byte("untouched foreign bytes"), 0600); err != nil {
						t.Fatal(err)
					}
				}
				if err := check(); err != nil {
					return err
				}
				checks++
				return renameExclusive(from, old, to, new)
			}
			result, err := applySkill(ctx, plan, root, gate, sharedSkillValidation(plan.name), ops)
			if kind == "unchanged" {
				if err != nil || !result.Committed || checks != 2 {
					t.Fatalf("result=%+v checks=%d err=%v", result, checks, err)
				}
				return
			}
			if err == nil || result.Committed || checks != 1 {
				t.Fatalf("unsafe publication: result=%+v checks=%d err=%v", result, checks, err)
			}
			if kind == "cancel" && !errors.Is(err, context.Canceled) {
				t.Fatal(err)
			}
			// Generated payload corruption is owned and may be cleaned. Every
			// source, reservation replacement and destination remains foreign.
			if peer != "" && kind != "tree" {
				body, readErr := os.ReadFile(peer)
				if readErr != nil || string(body) != "untouched foreign bytes" {
					t.Fatalf("foreign bytes changed: %q %v", body, readErr)
				}
			}
		})
	}
}
