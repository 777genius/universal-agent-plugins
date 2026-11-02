//go:build linux

package pluginkitairepo_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/gemini"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/installer"
)

func geminiCycleTempRecipe(t *testing.T) *geminiCycleRecipe {
	t.Helper()
	state := filepath.Join(t.TempDir(), "state")
	if err := os.Mkdir(state, 0700); err != nil {
		t.Fatal(err)
	}
	return &geminiCycleRecipe{StateRoot: state, Active: filepath.Join(state, "managed", "owned"), Data: filepath.Join(state, "plugin-data", "owned")}
}

func geminiCycleTempTree(t *testing.T, path string) map[string]geminiCycleRecord {
	t.Helper()
	rows, err := geminiCycleTree(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	return rows
}

func TestGeminiCycleActualInstallerDigestRefusalLeavesOnlyEmptyTemporaryParent(t *testing.T) {
	r := geminiCycleTempRecipe(t)
	root := filepath.Dir(r.StateRoot)
	source, caller := filepath.Join(root, "source"), filepath.Join(root, "caller")
	projectNativeRuntimeWrite(t, filepath.Join(source, "plugin.json"), []byte(`{"name":"inert-gemini","version":"1.0.0"}`), 0600)
	projectNativeRuntimeWrite(t, filepath.Join(caller, "keep"), []byte("caller authority"), 0600)
	before := geminiCycleTempTree(t, r.StateRoot)
	sourceBefore, callerBefore := geminiCycleTempTree(t, source), geminiCycleTempTree(t, caller)
	registry, err := clients.NewRegistry(gemini.New())
	if err != nil {
		t.Fatal(err)
	}
	engine, err := installer.New(installer.Config{StateRoot: r.StateRoot, TempRoot: filepath.Join(r.StateRoot, "installer-tmp"), Registry: registry})
	if err != nil {
		t.Fatal(err)
	}
	// No evaluator or trusted-local policy: exercise the real refusal and its
	// deferred snapshot removal, without a public installer or native child.
	digest, err := engine.LocalPackageTreeDigest(context.Background(), source)
	if !errors.Is(err, installer.ErrAssessmentRejected) || digest != "" {
		t.Fatal("actual unassessed package did not refuse", err)
	}
	after := geminiCycleTempTree(t, r.StateRoot)
	if len(before) != 1 || len(after) != 2 || after["installer-tmp"].Mode != 0040000|0700 {
		t.Fatal("installer digest left state, lock or snapshot residue")
	}
	if err := geminiCycleStateClosure(before, after, r); err != nil {
		t.Fatal("actual installer temporary parent refused", err)
	}
	if !reflect.DeepEqual(sourceBefore, geminiCycleTempTree(t, source)) || !reflect.DeepEqual(callerBefore, geminiCycleTempTree(t, caller)) {
		t.Fatal("digest refusal changed source or caller")
	}
}

func TestGeminiCycleTemporaryParentRejectsResidueAliasesReplacementAndCallerMutation(t *testing.T) {
	for _, kind := range []string{"retained", "both-absent", "new-child", "unchanged-child", "new-mode", "unchanged-mode", "file", "unchanged-file", "alias", "replacement", "deletion", "caller"} {
		t.Run(kind, func(t *testing.T) {
			r := geminiCycleTempRecipe(t)
			path := filepath.Join(r.StateRoot, "installer-tmp")
			if kind != "both-absent" && kind != "new-mode" {
				if err := os.Mkdir(path, 0700); err != nil {
					t.Fatal(err)
				}
			}
			if kind == "unchanged-child" {
				projectNativeRuntimeWrite(t, filepath.Join(path, "residue"), []byte("unknown"), 0600)
			}
			if kind == "unchanged-mode" {
				if err := os.Chmod(path, 0755); err != nil {
					t.Fatal(err)
				}
			}
			if kind == "unchanged-file" {
				if err := os.Remove(path); err != nil {
					t.Fatal(err)
				}
				projectNativeRuntimeWrite(t, path, []byte("file"), 0600)
			}
			projectNativeRuntimeWrite(t, filepath.Join(r.StateRoot, "caller"), []byte("held"), 0600)
			before := geminiCycleTempTree(t, r.StateRoot)
			switch kind {
			case "new-child":
				projectNativeRuntimeWrite(t, filepath.Join(path, "residue"), []byte("unknown"), 0600)
			case "new-mode":
				if err := os.Mkdir(path, 0755); err != nil {
					t.Fatal(err)
				}
			case "file", "alias", "deletion":
				if err := os.Remove(path); err != nil {
					t.Fatal(err)
				}
				switch kind {
				case "file":
					projectNativeRuntimeWrite(t, path, []byte("file"), 0600)
				case "alias":
					if err := os.Symlink(filepath.Dir(r.StateRoot), path); err != nil {
						t.Fatal(err)
					}
				}
			case "replacement":
				if err := os.Rename(path, filepath.Join(filepath.Dir(r.StateRoot), "old-temp")); err != nil {
					t.Fatal(err)
				}
				if err := os.Mkdir(path, 0700); err != nil {
					t.Fatal(err)
				}
			case "caller":
				if err := os.WriteFile(filepath.Join(r.StateRoot, "caller"), []byte("changed"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			after, err := geminiCycleTree(context.Background(), r.StateRoot)
			if kind == "alias" {
				if err == nil || after != nil {
					t.Fatal("aliased temporary parent produced a tree")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			err = geminiCycleStateClosure(before, after, r)
			want := kind == "retained" || kind == "both-absent"
			if (err == nil) != want {
				t.Fatal("temporary parent closure accepted wrong authority", err)
			}
		})
	}
}
