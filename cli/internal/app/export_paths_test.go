package app

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/777genius/plugin-kit-ai/cli/internal/pluginmanifest"
	"github.com/777genius/plugin-kit-ai/cli/internal/runtimecheck"
)

func TestLauncherBundlePathsIncludesCmdVariant(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	mustWritePublicationSourceFile(t, root, "bin/demo", "#!/bin/sh\n")
	mustWritePublicationSourceFile(t, root, "bin/demo.cmd", "@echo off\r\n")

	got := launcherBundlePaths(root, "./bin/demo")
	want := []string{"bin/demo", "bin/demo.cmd"}
	if !slices.Equal(got, want) {
		t.Fatalf("launcherBundlePaths() = %v, want %v", got, want)
	}
}

func TestExportExcludedPathsIncludesManagedPythonEnvWithinRoot(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	project := runtimecheck.Project{
		Python: runtimecheck.PythonShape{
			ReadySource:   runtimecheck.PythonEnvSourceManagerOwned,
			ProbedEnvPath: filepath.Join(root, ".venv-managed"),
		},
	}
	got := exportExcludedPaths(root, project)
	if !slices.Contains(got, ".venv-managed") {
		t.Fatalf("exportExcludedPaths() = %v, want .venv-managed", got)
	}
}

func TestExportFileListRejectsSymlinkedPath(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	target := filepath.Join(root, "real.txt")
	if err := os.WriteFile(target, []byte("ok"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, filepath.Join(root, "linked.txt")); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}

	_, err := exportFileList(root, pluginmanifest.PackageGraph{SourceFiles: []string{"linked.txt"}}, runtimecheck.Project{}, nil)
	if err == nil || err.Error() != "export refuses symlinked path linked.txt" {
		t.Fatalf("error = %v", err)
	}
}
