package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestManagedStdioMainSubprocess(t *testing.T) {
	if os.Getenv("UAP_EARLY_MAIN_FIXTURE") != "1" {
		return
	}
	os.Args = []string{"agentplugins", "--internal-stdio-v1"}
	main()
}

func TestManagedStdioDispatchPrecedesAllUserState(t *testing.T) {
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	state := filepath.Join(root, "must-not-exist")
	cmd := exec.Command(exe, "-test.run=^TestManagedStdioMainSubprocess$")
	cmd.Env = append(os.Environ(), "UAP_EARLY_MAIN_FIXTURE=1", "HOME="+state, "AGENTPLUGINS_HOME="+state, "AGENTPLUGINS_DIRECTORY_ORIGIN=not-a-url")
	var out, stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr
	err = cmd.Run()
	if ee, ok := err.(*exec.ExitError); !ok || ee.ExitCode() != 126 {
		t.Fatalf("wrong private mode error: %v %s", err, stderr.String())
	}
	if out.Len() != 0 || !bytes.Contains(stderr.Bytes(), []byte("managed stdio:")) {
		t.Fatalf("private stdout/stderr = %q / %q", out.String(), stderr.String())
	}
	if _, err := os.Stat(state); !os.IsNotExist(err) {
		t.Fatalf("private mode touched user state: %v", err)
	}
}
