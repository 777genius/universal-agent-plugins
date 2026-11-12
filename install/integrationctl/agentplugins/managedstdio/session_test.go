package managedstdio

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestSessionRejectsInvalidProcessBeforeChangingDirectory(t *testing.T) {
	before, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for _, example := range []struct {
		command, cwd string
		args, env    []string
	}{
		{"relative", before, []string{"relative"}, nil},
		{before, "relative", []string{before}, nil},
		{before, before, nil, nil},
		{before, before, []string{"other"}, nil},
		{before, before, []string{before}, []string{"BAD=\x00"}},
	} {
		if err := StartSession(example.command, example.args, example.cwd, example.env); err == nil {
			t.Fatal("invalid native start accepted")
		}
	}
	after, _ := os.Getwd()
	if before != after {
		t.Fatal("invalid native start changed cwd")
	}
}

func TestSessionPreservesInheritedStdioAndExplicitEnvironment(t *testing.T) {
	if os.Getenv("UAP_SESSION_CHILD") == "1" {
		cwd := os.Getenv("UAP_SESSION_CWD")
		if err := StartSession(os.Args[0], []string{os.Args[0], "-test.run=^TestSessionTargetChild$"}, cwd,
			[]string{"UAP_SESSION_TARGET=1", "UAP_SESSION_VALUE=selected", "UAP_SESSION_CWD=" + cwd, "SYSTEMROOT=" + os.Getenv("SYSTEMROOT")}); err != nil {
			t.Fatal(err)
		}
		return
	}
	if !Supported() {
		t.Skip("no native session process boundary on this platform")
	}
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	cwd := t.TempDir()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, executable, "-test.run=^TestSessionPreservesInheritedStdioAndExplicitEnvironment$")
	cmd.Env = append(os.Environ(), "UAP_SESSION_CHILD=1", "UAP_SESSION_CWD="+cwd, "UAP_SESSION_VALUE=original")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("native boundary: %v\n%s", err, output)
	}
	if !strings.Contains(string(output), "native-session-ok") {
		t.Fatal("child did not inherit stdio", string(output))
	}
}

func TestSessionTargetChild(t *testing.T) {
	if os.Getenv("UAP_SESSION_TARGET") != "1" {
		return
	}
	cwd, err := os.Getwd()
	if err != nil || os.Getenv("UAP_SESSION_VALUE") != "selected" {
		t.Fatal("native environment differs", err)
	}
	want := os.Getenv("UAP_SESSION_CWD")
	if runtime.GOOS == "darwin" {
		want, err = filepath.EvalSymlinks(want)
	}
	if err != nil || cwd != want {
		t.Fatalf("cwd differs: %s != %s", cwd, want)
	}
	if os.Getenv("UAP_SESSION_CHILD") != "" {
		t.Fatal("unselected parent environment leaked")
	}
	_, _ = os.Stdout.WriteString("native-session-ok\n")
}
