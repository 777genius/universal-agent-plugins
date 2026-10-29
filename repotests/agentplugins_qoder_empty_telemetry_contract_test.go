//go:build linux

package pluginkitairepo_test

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"golang.org/x/sys/unix"
)

func qoderTelemetryFixture(t *testing.T) (string, *qoderOriginalMetadata, *qoderEmptyTelemetry) {
	t.Helper()
	root := qoderMetadataFixture(t)
	metadata, _ := qoderMetadataAuthor(t, root)
	recipe, err := qoderAuthorEmptyTelemetry(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	return root, metadata, recipe
}

func qoderTelemetryPath(root string, level int) string {
	for _, name := range qoderTelemetryNames[:level+1] {
		root = filepath.Join(root, name)
	}
	return root
}

func TestQoderEmptyTelemetryAuthorsExactOriginalChainAndRetainsMetadata(t *testing.T) {
	root, metadata, recipe := qoderTelemetryFixture(t)
	for i := range qoderTelemetryNames {
		info, err := os.Lstat(qoderTelemetryPath(root, i))
		if err != nil || !qoderLogOwned(info, true) || info.Mode() != os.ModeDir|0700 || !os.SameFile(recipe.directories[i], info) {
			t.Fatal("authored directory mode/owner/identity differs")
		}
	}
	before, err := projectNativeCLIQoderProfile(root)
	if err != nil {
		t.Fatal(err)
	}
	for range 3 {
		if recipe.check() != nil || metadata.check() != nil {
			t.Fatal("fresh membership check lost exact recipe or original metadata")
		}
	}
	marker := filepath.Join(qoderTelemetryPath(root, 2), "inert-marker-never-native")
	projectNativeRuntimeWrite(t, marker, []byte("inert private body; never exported"), 0600)
	if recipe.check() == nil {
		t.Fatal("leftover marker became a new accepted baseline")
	}
	if err := os.Remove(marker); err != nil {
		t.Fatal(err)
	}
	if recipe.check() != nil || metadata.check() != nil {
		t.Fatal("create/remove marker incorrectly pinned directory timestamps")
	}
	after, err := projectNativeCLIQoderProfile(root)
	if err != nil || !reflect.DeepEqual(before, after) {
		t.Fatal("empty directory recipe changed the full profile contract")
	}
	projectNativeRuntimeWrite(t, filepath.Join(root, "logs", "unknown.log"), []byte("unapproved log"), 0600)
	after, err = projectNativeCLIQoderProfile(root)
	if err != nil || reflect.DeepEqual(before, after) || recipe.check() != nil {
		t.Fatal("telemetry recipe became a logging exception or whole-profile baseline")
	}
}

func TestQoderEmptyTelemetryRefusesEveryUnknownSiblingAndSpecialChild(t *testing.T) {
	for level := range qoderTelemetryNames {
		for _, kind := range []string{"file", "directory", "symlink", "fifo"} {
			t.Run(qoderTelemetryNames[level]+"/"+kind, func(t *testing.T) {
				root, _, recipe := qoderTelemetryFixture(t)
				path := filepath.Join(qoderTelemetryPath(root, level), "private-unknown")
				var err error
				switch kind {
				case "file":
					err = os.WriteFile(path, []byte("PRIVATE-BODY-must-not-be-read"), 0000)
				case "directory":
					err = os.Mkdir(path, 0700)
				case "symlink":
					err = os.Symlink(filepath.Join(root, "settings.json"), path)
				case "fifo":
					err = unix.Mkfifo(path, 0600)
				}
				if err != nil {
					t.Fatal(err)
				}
				if recipe.check() == nil {
					t.Fatal("unknown child entered exact initial membership")
				}
			})
		}
	}
}

func TestQoderEmptyTelemetryRefusesModeAndCoherentParentReplacement(t *testing.T) {
	for level := range qoderTelemetryNames {
		for _, kind := range []string{"mode", "replacement", "alias"} {
			t.Run(qoderTelemetryNames[level]+"/"+kind, func(t *testing.T) {
				root, _, recipe := qoderTelemetryFixture(t)
				path := qoderTelemetryPath(root, level)
				retired := filepath.Join(filepath.Dir(root), "retained-original")
				if kind == "mode" {
					if err := os.Chmod(path, 0755); err != nil {
						t.Fatal(err)
					}
				} else {
					if err := os.Rename(path, retired); err != nil {
						t.Fatal(err)
					}
					if kind == "alias" {
						if err := os.Symlink(retired, path); err != nil {
							t.Fatal(err)
						}
					} else {
						if err := os.Mkdir(path, 0700); err != nil {
							t.Fatal(err)
						}
						if level+1 < len(qoderTelemetryNames) {
							name := qoderTelemetryNames[level+1]
							if err := os.Rename(filepath.Join(retired, name), filepath.Join(path, name)); err != nil {
								t.Fatal(err)
							}
						}
					}
				}
				if recipe.check() == nil {
					t.Fatal("mode/alias/parent substitution preserving children was accepted")
				}
			})
		}
	}
}

func TestQoderEmptyTelemetryRefusesExistingTargetsAndOriginalAncestorSubstitution(t *testing.T) {
	for _, kind := range []string{"directory", "file", "symlink", "fifo"} {
		root := qoderMetadataFixture(t)
		path := filepath.Join(root, "tmp")
		var err error
		switch kind {
		case "directory":
			err = os.Mkdir(path, 0700)
		case "file":
			err = os.WriteFile(path, []byte("preexisting caller authority"), 0600)
		case "symlink":
			err = os.Symlink(t.TempDir(), path)
		case "fifo":
			err = unix.Mkfifo(path, 0600)
		}
		if err != nil {
			t.Fatal(err)
		}
		before, err := os.Lstat(path)
		if err != nil {
			t.Fatal(err)
		}
		if got, err := qoderAuthorEmptyTelemetry(context.Background(), root); err == nil || got != nil {
			t.Fatal("preexisting caller target was adopted")
		}
		after, err := os.Lstat(path)
		if err != nil || !projectNativeRuntimeStableStat(before.Sys(), after.Sys()) {
			t.Fatal("refusal changed the preexisting target")
		}
	}
	parent := filepath.Join(t.TempDir(), "owned-parent")
	root := filepath.Join(parent, ".qoder")
	if err := os.MkdirAll(root, 0700); err != nil {
		t.Fatal(err)
	}
	recipe, err := qoderAuthorEmptyTelemetry(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(parent, parent+"-retired"); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(parent, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(filepath.Join(parent+"-retired", ".qoder"), root); err != nil {
		t.Fatal(err)
	}
	if recipe.check() == nil {
		t.Fatal("original ancestor replacement preserving the entire profile was accepted")
	}
}

func TestQoderEmptyTelemetryUsesOriginalContextAndCannotHideMetadataDrift(t *testing.T) {
	root, metadata, recipe := qoderTelemetryFixture(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	recipe.ctx = ctx
	if recipe.check() == nil {
		t.Fatal("canceled original work context allowed a new telemetry check")
	}
	recipe.ctx = context.Background()
	projectNativeRuntimeWrite(t, filepath.Join(root, ".auth", "private-extra"), []byte("unapproved auth sibling"), 0600)
	if recipe.check() != nil || metadata.check() == nil {
		t.Fatal("telemetry recipe hid the unchanged closed auth membership failure")
	}
}
