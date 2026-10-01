package clientdetect

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func nativeOpenCodeFixture(t *testing.T, version, mode string) string {
	t.Helper()
	// All executed fixture bytes and probe working directories live in this
	// checkout, even when a caller has not supplied TMPDIR for the test command.
	root, err := os.MkdirTemp(".", "TEST-opencode-")
	if err != nil {
		t.Fatal(err)
	}
	root, err = filepath.Abs(root)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(root) })
	t.Setenv("TMPDIR", root)
	binary := filepath.Join(root, "opencode")
	if runtime.GOOS == "windows" {
		binary += ".exe"
	}
	command := exec.Command("go", "build", "-ldflags", "-X main.version="+version+" -X main.mode="+mode, "-o", binary, "testdata/opencode_probe.go")
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("build native fixture: %s %v", output, err)
	}
	return binary
}

// Regression: ambient PATH or user environment cannot replace the explicit V2
// target, nor can a successful CLI probe read a real project or inherit secrets.
func TestOpenCodeTargetExplicitAuthorityAndIsolation(t *testing.T) {
	v1 := nativeOpenCodeFixture(t, "1.18.33", "ok")
	v2 := nativeOpenCodeFixture(t, "2.0.21", "prefix")
	t.Setenv("PATH", filepath.Dir(v1))
	t.Setenv("HOME", "/fixture-not-inherited")
	t.Setenv("AGENTPLUGINS_TEST_CREDENTIAL", "not-inherited")
	env := []string{"PATH=" + filepath.Dir(v1)}
	evidence, err := ProbeOpenCodeTarget(context.Background(), ProbeTarget{Executable: v2, Environment: env})
	if err != nil || evidence.Version != "2.0.21" || evidence.ProbeStatus != "ok" || evidence.ExecutableIdentity == "" {
		t.Fatalf("%+v %v", evidence, err)
	}
	body, err := os.ReadFile(filepath.Join(filepath.Dir(v2), "observed.json"))
	if err != nil {
		t.Fatal(err)
	}
	var observed struct {
		CWD   string
		Env   []string
		Stdin string
		Args  []string
	}
	if err := json.Unmarshal(body, &observed); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(observed.CWD, filepath.Dir(v2)+string(os.PathSeparator)) || observed.Stdin != "" || !reflect.DeepEqual(observed.Args, []string{"--version"}) {
		t.Fatalf("isolation: %+v", observed)
	}
	if !reflect.DeepEqual(observed.Env, env) && runtime.GOOS != "windows" {
		t.Fatalf("environment: %v", observed.Env)
	}
	public, _ := json.Marshal(evidence)
	if strings.Contains(string(public), evidence.ExecutableIdentity) || strings.Contains(string(public), v2) {
		t.Fatalf("private authority leaked: %s", public)
	}
}

// Regression: failed/malformed/limited/cancelled output cannot normalize into a
// usable version. stdout alone is authoritative, with a combined stream limit.
func TestOpenCodeTargetFailureStatuses(t *testing.T) {
	for _, tc := range []struct{ mode, status string }{{"failed", "failed"}, {"malformed", "malformed"}, {"loose", "malformed"}, {"stderr", "malformed"}, {"limit", "output_limit"}, {"slow", "timed_out"}} {
		t.Run(tc.mode, func(t *testing.T) {
			binary := nativeOpenCodeFixture(t, "1.18.33", tc.mode)
			target := ProbeTarget{Executable: binary, Environment: []string{"PATH="}}
			if tc.mode == "slow" {
				target.Timeout = 200 * time.Millisecond
			}
			evidence, err := ProbeOpenCodeTarget(context.Background(), target)
			if err == nil || evidence.ProbeStatus != tc.status || evidence.Version != "" || strings.Contains(err.Error(), binary) {
				t.Fatalf("%+v %v", evidence, err)
			}
			if tc.mode == "slow" && !errors.Is(err, context.DeadlineExceeded) {
				t.Fatalf("lost timeout: %v", err)
			}
		})
	}
	root, err := filepath.Abs(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	evidence, err := ProbeOpenCodeTarget(context.Background(), ProbeTarget{Executable: filepath.Join(root, "absent"), Environment: []string{"PATH="}})
	if err == nil || evidence.ProbeStatus != "absent" {
		t.Fatalf("%+v %v", evidence, err)
	}
	evidence, err = ProbeOpenCodeTarget(context.Background(), ProbeTarget{})
	if err != nil || evidence.ProbeStatus != "not_requested" {
		t.Fatalf("%+v %v", evidence, err)
	}
	binary := nativeOpenCodeFixture(t, "1.18.33", "ok")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	evidence, err = ProbeOpenCodeTarget(ctx, ProbeTarget{Executable: binary, Environment: []string{"PATH="}})
	if !errors.Is(err, context.Canceled) || evidence.ProbeStatus != "timed_out" {
		t.Fatalf("cancel: %+v %v", evidence, err)
	}
}

// Regression: equal output cannot conceal changed native bytes or a symlink
// redirect. Unsupported wrappers fail before execution, including unchanged
// wrapper bytes whose interpreter/runtime has been swapped.
func TestOpenCodeTargetIdentityAndWrapperRestriction(t *testing.T) {
	binary := nativeOpenCodeFixture(t, "1.18.33", "ok")
	replacement := nativeOpenCodeFixture(t, "1.18.33", "prefix")
	target := ProbeTarget{Executable: binary, Environment: []string{"PATH="}}
	before, err := ProbeOpenCodeTarget(context.Background(), target)
	if err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(replacement)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(binary, body, 0700); err != nil {
		t.Fatal(err)
	}
	after, err := ProbeOpenCodeTarget(context.Background(), target)
	if err != nil || after.Version != before.Version || after.ExecutableIdentity == before.ExecutableIdentity {
		t.Fatalf("same-version change: %+v %+v %v", before, after, err)
	}
	if runtime.GOOS == "windows" {
		return
	}
	link := filepath.Join(filepath.Dir(binary), "selected")
	if err := os.Symlink(binary, link); err != nil {
		t.Fatal(err)
	}
	target.Executable = link
	before, err = ProbeOpenCodeTarget(context.Background(), target)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(link); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(replacement, link); err != nil {
		t.Fatal(err)
	}
	after, err = ProbeOpenCodeTarget(context.Background(), target)
	if err != nil || before.ExecutableIdentity == after.ExecutableIdentity {
		t.Fatalf("symlink identity: %v", err)
	}
	wrapper := filepath.Join(filepath.Dir(binary), "wrapper")
	if err := os.WriteFile(wrapper, []byte("#!/bin/sh\nexec \""+link+"\" --version\n"), 0700); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		evidence, err := ProbeOpenCodeTarget(context.Background(), ProbeTarget{Executable: wrapper, Environment: []string{"PATH="}})
		if !errors.Is(err, ErrUnverifiedProbeTarget) || evidence.Reason != "host_target_unverified" {
			t.Fatalf("wrapper: %+v %v", evidence, err)
		}
		if err := os.WriteFile(binary, []byte("swapped runtime"), 0700); err != nil {
			t.Fatal(err)
		}
	}
}

// Regression: a nested directory link followed by .. cannot make a native
// decoy authorize execution of an actual script payload at another location.
func TestOpenCodeTargetNestedLinkRejectsActualWrapper(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink fixture requires Unix")
	}
	binary := nativeOpenCodeFixture(t, "1.18.33", "ok")
	root := filepath.Dir(binary)
	if err := os.MkdirAll(filepath.Join(root, "other", "sub"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("other/sub", filepath.Join(root, "dirlink")); err != nil {
		t.Fatal(err)
	}
	selected := filepath.Join(root, "selected")
	if err := os.Symlink("dirlink/../opencode", selected); err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(root, "wrapper-ran")
	if err := os.WriteFile(filepath.Join(root, "other", "opencode"), []byte("#!/bin/sh\necho executed > \""+marker+"\"\necho 1.18.33\n"), 0700); err != nil {
		t.Fatal(err)
	}
	evidence, err := ProbeOpenCodeTarget(context.Background(), ProbeTarget{Executable: selected, Environment: []string{"PATH="}})
	if !errors.Is(err, ErrUnverifiedProbeTarget) || evidence.Reason != "host_target_unverified" {
		t.Fatalf("actual wrapper authorized: %+v %v", evidence, err)
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("actual wrapper ran: %v", err)
	}
}

// Regression: caller-owned env slices cannot alias the pinned snapshot, and
// duplicate/case-equivalent PATH cannot create two competing authorities.
func TestOpenCodeTargetEnvironmentValidation(t *testing.T) {
	for _, env := range [][]string{nil, {"PATH=x", "Path=y"}, {"PATH=x", "HOME=y"}, {"PATH=x\x00"}, {"PATH"}} {
		if _, err := CopyOpenCodeProbeEnvironment(env); !errors.Is(err, ErrInvalidProbeTarget) {
			t.Fatalf("accepted %v", env)
		}
	}
	env := []string{"PATH="}
	copied, err := CopyOpenCodeProbeEnvironment(env)
	if err != nil {
		t.Fatal(err)
	}
	env[0] = "PATH=changed"
	if copied[0] != "PATH=" {
		t.Fatal("copy aliases caller")
	}
}

// Regression: presence of the separate V2 binary must not rename it to
// opencode, replace the primary candidate, or start an observational probe.
func TestOpenCodePresencePreservesSeparateCandidates(t *testing.T) {
	for _, primary := range []bool{false, true} {
		home := t.TempDir()
		binaries := map[string]string{"opencode2": filepath.Join(home, "bin", "opencode2")}
		if primary {
			binaries["opencode"] = filepath.Join(home, "bin", "opencode")
		}
		detector := testDetector(home, binaries)
		detector.ProbeVersion = func(context.Context, string) (string, error) {
			t.Fatal("presence detection executed a version probe")
			return "", nil
		}
		detected, err := detector.Detect(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		client := clientOf(detected, domain.ClientOpenCode)
		if client.Status != domain.DetectionDetected || client.ExecutablePath != binaries["opencode"] || client.Version != "" || !surfaceDetected(client.Surfaces, "opencode2_cli") || surfaceDetected(client.Surfaces, "opencode_cli") != primary {
			t.Fatalf("candidate authority: %+v", client)
		}
	}
}

// Regression: a caller cannot lengthen the frozen ten-second maximum, and
// cancellation deadlines may shorten it without losing the timeout status.
func TestOpenCodeTargetTimeoutCannotBeExtended(t *testing.T) {
	binary := nativeOpenCodeFixture(t, "1.18.33", "slow")
	for _, deadline := range []time.Duration{0, 150 * time.Millisecond} {
		ctx := context.Background()
		cancel := func() {}
		limit := 12 * time.Second
		if deadline != 0 {
			ctx, cancel = context.WithTimeout(ctx, deadline)
			limit = 2 * time.Second
		}
		start := time.Now()
		evidence, err := ProbeOpenCodeTarget(ctx, ProbeTarget{Executable: binary, Environment: []string{"PATH="}, Timeout: 30 * time.Second})
		cancel()
		if !errors.Is(err, context.DeadlineExceeded) || evidence.ProbeStatus != "timed_out" || time.Since(start) > limit {
			t.Fatalf("probe exceeded %s: %+v %v", limit, evidence, err)
		}
	}
}
