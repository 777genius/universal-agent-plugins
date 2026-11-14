//go:build linux

package main

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/sys/unix"
)

func TestTrustedFinishPublicCompletionMatchesRejectedSummary(t *testing.T) {
	root := t.TempDir()
	output, err := os.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = output.Close() })
	var now unix.Timespec
	if err := unix.ClockGettime(unix.CLOCK_MONOTONIC, &now); err != nil {
		t.Fatal(err)
	}
	pin := strings.Repeat("a", 64)
	in := inputRequest{Mode: trustedProjectMode, SourceTree: strings.Repeat("b", 40), StartNS: now.Nano(), OutputRoot: root, UID: os.Getuid(), GID: os.Getgid(),
		Trusted: &trustedInput{ToolsRoot: "/inert/tools", ToolsManifestSHA256: pin, InstallerPath: "/inert/installer", InstallerSHA256: pin, ProbePath: "/inert/probe", ProbeSHA256: pin, ScannerPath: "/inert/scanner", ScannerSHA256: pin, ScannerArchiveSHA256: trustedScannerArchive}}
	c := &controller{request: sealedRequest{Input: in},
		trustedState: &trustedLifecycle{Complete: true}, files: map[string][]byte{}, output: output}
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	stdout := os.Stdout
	os.Stdout = writer
	t.Cleanup(func() { os.Stdout = stdout; _ = reader.Close(); _ = writer.Close() })
	finishErr := c.finish(nil, nil, nil, nil)
	os.Stdout = stdout
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if finishErr != nil {
		t.Fatal(finishErr)
	}
	public, err := io.ReadAll(reader)
	if err != nil {
		t.Fatal(err)
	}
	stored, err := os.ReadFile(filepath.Join(root, "vscode-workspace-controller.json"))
	if err != nil {
		t.Fatal(err)
	}
	var status, summary map[string]any
	if err := json.Unmarshal(public, &status); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(stored, &summary); err != nil {
		t.Fatal(err)
	}
	if status["collection_complete"] != false || summary["collection_complete"] != false || status["cleanup_complete"] != true {
		t.Fatal("missing native stages were promoted or independent completed cleanup was lost", status, summary)
	}
}
