package pluginkitai_test

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"
)

type localObservation struct {
	Callback       string `json:"callback"`
	Calls          int    `json:"calls"`
	NativeName     string `json:"native_name"`
	Timestamp      string `json:"timestamp"`
	CWD            string `json:"cwd"`
	SessionID      string `json:"session_id"`
	TranscriptPath string `json:"transcript_path"`
	Known          bool   `json:"known"`
	Active         bool   `json:"active"`
	Eligible       bool   `json:"eligible"`
	AgentID        string `json:"agent_id"`
	AgentType      string `json:"agent_type"`
	Code           int    `json:"code"`
	Stdout         string `json:"stdout"`
	Stderr         string `json:"stderr"`
}

type localConsumer struct {
	binary string
	env    []string
}

func buildLocalConsumer(t *testing.T) localConsumer {
	t.Helper()
	root, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	home := filepath.Join(dir, "TEST-HOME")
	if err := os.Mkdir(home, 0o700); err != nil {
		t.Fatal(err)
	}
	// Use the running Go test toolchain, never an ambient/downloaded toolchain.
	goBinary := filepath.Join(runtime.GOROOT(), "bin", "go")
	env := []string{"HOME=" + home, "TMPDIR=" + dir, "GOTMPDIR=" + dir,
		"GOTOOLCHAIN=local", "GOWORK=off", "GOPROXY=off", "GOSUMDB=off", "GOMAXPROCS=2",
		"PATH=" + filepath.Dir(goBinary) + string(os.PathListSeparator) + "/usr/bin:/bin"}
	for _, key := range []string{"GOCACHE", "GOMODCACHE"} {
		value := os.Getenv(key)
		if value == "" {
			value = filepath.Join(dir, key)
		}
		env = append(env, key+"="+value)
	}
	mod, err := os.ReadFile("testdata/vscodelocal-observer/go.mod")
	if err != nil {
		t.Fatal(err)
	}
	mod = append(mod, []byte("\nreplace github.com/777genius/plugin-kit-ai/sdk => "+strconv.Quote(root)+"\n")...)
	modfile := filepath.Join(dir, "go.mod")
	if err := os.WriteFile(modfile, mod, 0o600); err != nil {
		t.Fatal(err)
	}
	binary := filepath.Join(dir, "TEST-local-observer")
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	args := []string{"build", "-p", "2", "-modfile", modfile, "-o", binary}
	if cursorConsumerRace {
		args = append(args, "-race")
	}
	cmd := exec.CommandContext(ctx, goBinary, append(args, ".")...)
	cmd.Dir, cmd.Env = filepath.Join(root, "testdata/vscodelocal-observer"), env
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build independent public-module executable: %v\n%s", err, out)
	}
	return localConsumer{binary: binary, env: []string{"HOME=" + home, "TMPDIR=" + dir}}
}

func (c localConsumer) run(t *testing.T, selector, mode, input string) localObservation {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, c.binary, selector, mode)
	cmd.Env, cmd.Stdin = c.env, strings.NewReader(input)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil || stderr.Len() != 0 {
		t.Fatalf("TEST consumer: %v, stderr=%q", err, stderr.String())
	}
	var got localObservation
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatalf("TEST consumer report: %v", err)
	}
	return got
}
