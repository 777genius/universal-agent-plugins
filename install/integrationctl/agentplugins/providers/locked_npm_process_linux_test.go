//go:build linux

package providers

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"golang.org/x/sys/unix"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/process"
)

// These regressions require a real Linux proc children interface and pidfds.
// Existing process runner tests own Darwin process-group and Windows Job proof.
func TestLockedNPMCancellationStopsInheritedStderrHelperBeforeRuntimeCleanup(t *testing.T) {
	for _, mode := range []string{"cancel", "deadline"} {
		t.Run(mode, func(t *testing.T) {
			requireLockedNPMProcessPrerequisites(t)
			root := installLockedNPMProcessFixture(t)
			socket := listenLockedNPMFixture(t)
			manager, envelope, plan, dataPath, _ := lockedRuntimeFixture(t)
			manager.RunNPM = nil // Exercise the production command and supervisor.
			ctx, cancel := context.WithCancel(t.Context())
			want := context.Canceled
			if mode == "deadline" {
				cancel()
				ctx, cancel = context.WithTimeout(t.Context(), 15*time.Second)
				want = context.DeadlineExceeded
			}
			defer cancel()
			done := make(chan error, 1)
			go func() { done <- manager.PrepareRuntime(ctx, envelope, plan, dataPath) }()
			completed := false
			t.Cleanup(func() {
				cancel()
				if !completed {
					select {
					case <-done:
					case <-time.After(5 * time.Second):
						t.Error("fixture preparation did not finish during test cleanup")
					}
				}
			})
			// One readiness budget covers both handoffs, independently of the
			// longer real deadline. An unready fixture cannot prove cancellation.
			if err := socket.SetReadDeadline(time.Now().Add(5 * time.Second)); err != nil {
				t.Fatal(err)
			}
			handles := make(map[string]int, 2)
			for range 2 {
				role, fd := receiveLockedNPMFixturePIDFD(t, socket)
				if _, duplicate := handles[role]; duplicate {
					t.Fatalf("duplicate fake npm %s readiness", role)
				}
				handles[role] = fd
			}
			if mode == "cancel" {
				cancel()
			}
			var runErr error
			select {
			case <-ctx.Done():
			case runErr = <-done:
				completed = true
				// Cancellation and completion may both be ready after this
				// goroutine is rescheduled. Preserve the consumed outcome.
				if ctx.Err() == nil {
					t.Fatalf("npm preparation stopped before cancellation: %v", runErr)
				}
			}
			if !completed {
				select {
				case runErr = <-done:
					completed = true
				case <-time.After(6 * time.Second):
					t.Fatal("npm cancellation hung on an inherited stderr descriptor")
				}
			}
			if !errors.Is(runErr, want) {
				t.Fatalf("PrepareRuntime error = %v, want %v", runErr, want)
			}
			if mode == "cancel" && strings.Contains(runErr.Error(), "timed out") {
				t.Fatalf("cancellation mislabeled as timeout: %v", runErr)
			}
			for name, fd := range handles {
				if !lockedNPMPIDFDExited(t, fd) {
					t.Errorf("%s remained alive after preparation returned", name)
				}
			}
			assertNoPreparedNPMRuntime(t, dataPath)
			// The helper is armed to recreate a file in npm's cache after the
			// caller returns. It must already be dead before this trigger.
			if err := os.WriteFile(filepath.Join(root, "mutate"), nil, 0o600); err != nil {
				t.Fatal(err)
			}
			time.Sleep(50 * time.Millisecond)
			if _, err := os.Stat(filepath.Join(dataPath, "npm-cache", "late-mutation")); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("helper mutated cache after cancellation: %v", err)
			}
			t.Setenv("UAP_LOCKED_NPM_FIXTURE_MODE", "success")
			if err := manager.PrepareRuntime(t.Context(), envelope, plan, dataPath); err != nil {
				t.Fatalf("real-command retry after %s: %v", mode, err)
			}
		})
	}
}

// This handoff check needs no proc-children supervisor, so local environments
// can verify descriptor ownership even when the full cancellation E2E must skip.
func TestLockedNPMFixtureTransferredPIDFDStopsItsOriginatingProcess(t *testing.T) {
	requireLockedNPMPIDFD(t)
	installLockedNPMProcessFixture(t)
	socket := listenLockedNPMFixture(t)
	t.Setenv("UAP_LOCKED_NPM_FIXTURE_CHILD", "1")
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestLockedNPMProcessFixture$")
	command.Env = os.Environ()
	command.Stdout, command.Stderr = os.Stdout, os.Stderr
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	waited := false
	t.Cleanup(func() {
		_ = command.Process.Kill()
		if !waited {
			_ = command.Wait()
		}
	})
	if err := socket.SetReadDeadline(time.Now().Add(5 * time.Second)); err != nil {
		t.Fatal(err)
	}
	role, fd := receiveLockedNPMFixturePIDFD(t, socket)
	if role != "child" {
		t.Fatalf("fixture transferred role %q, want child", role)
	}
	if err := unix.PidfdSendSignal(fd, unix.SIGKILL, nil, 0); err != nil {
		t.Fatal(err)
	}
	err := command.Wait()
	waited = true
	if err == nil || !lockedNPMPIDFDExited(t, fd) {
		t.Fatalf("transferred pidfd did not stop its originating process: %v", err)
	}
}

func TestRunLockedNPMRealCommandPreservesArgumentsEnvironmentAndExitStatus(t *testing.T) {
	requireLockedNPMProcessPrerequisites(t)
	installLockedNPMProcessFixture(t)
	t.Setenv("npm_config_proxy", "http://must-not-reach-fixture.invalid")
	t.Setenv("NODE_OPTIONS", "--must-not-reach-fixture")
	t.Setenv("UAP_LOCKED_NPM_EXPECT_OMIT_OPTIONAL", "1")
	for _, mode := range []string{"success", "failure"} {
		t.Run(mode, func(t *testing.T) {
			t.Setenv("UAP_LOCKED_NPM_FIXTURE_MODE", mode)
			err := runLockedNPM(t.Context(), t.TempDir(), t.TempDir(), true)
			if mode == "success" {
				if err != nil {
					t.Fatalf("real npm shim command: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), "exited with code 23") ||
				!strings.HasSuffix(err.Error(), "final npm diagnostic") || len(err.Error()) > 1400 {
				t.Fatalf("real npm shim failure = %v", err)
			}
		})
	}
}

func requireLockedNPMProcessPrerequisites(t *testing.T) {
	t.Helper()
	if err := (process.OS{}).OrdinaryPreflight(); err != nil {
		if errors.Is(err, process.ErrOrdinaryContainmentUnsupported) {
			if os.Getenv("RUNNER_ENVIRONMENT") == "github-hosted" {
				t.Fatalf("GitHub-hosted npm regression requires qualified Linux process containment: %v", err)
			}
			t.Skipf("real npm cancellation requires qualified Linux process containment: %v", err)
		}
		t.Fatalf("Linux process containment preflight: %v", err)
	}
	requireLockedNPMPIDFD(t)
}

func requireLockedNPMPIDFD(t *testing.T) {
	t.Helper()
	fd, err := unix.PidfdOpen(os.Getpid(), 0)
	if err != nil {
		if os.Getenv("RUNNER_ENVIRONMENT") == "github-hosted" {
			t.Fatalf("GitHub-hosted npm regression requires stable pidfd handles: %v", err)
		}
		t.Skipf("real npm cancellation requires stable pidfd handles: %v", err)
	}
	_ = unix.Close(fd)
}

func installLockedNPMProcessFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	bin := filepath.Join(root, "bin")
	if err := os.Mkdir(bin, 0o700); err != nil {
		t.Fatal(err)
	}
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	quoted := "'" + strings.ReplaceAll(executable, "'", "'\"'\"'") + "'"
	script := "#!/bin/sh\nexec " + quoted + " -test.run=^TestLockedNPMProcessFixture$ -- \"$@\"\n"
	if err := os.WriteFile(filepath.Join(bin, "npm"), []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("UAP_LOCKED_NPM_FIXTURE_ROOT", root)
	t.Setenv("UAP_LOCKED_NPM_FIXTURE_MODE", "hang")
	t.Setenv("UAP_LOCKED_NPM_EXPECT_OMIT_OPTIONAL", "0")
	return root
}

func listenLockedNPMFixture(t *testing.T) *net.UnixConn {
	t.Helper()
	// Linux abstract addresses avoid sockaddr_un's pathname length limit;
	// the random name keeps each fixture's descriptor transfer private.
	address := &net.UnixAddr{Name: "@uap-npm-" + rand.Text(), Net: "unixgram"}
	socket, err := net.ListenUnixgram("unixgram", address)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = socket.Close() })
	t.Setenv("UAP_LOCKED_NPM_FIXTURE_SOCKET", address.Name)
	return socket
}

func receiveLockedNPMFixturePIDFD(t *testing.T, socket *net.UnixConn) (string, int) {
	t.Helper()
	body, control := make([]byte, 16), make([]byte, unix.CmsgSpace(4))
	n, controlN, flags, _, err := socket.ReadMsgUnix(body, control)
	if err != nil {
		t.Fatalf("fake npm did not transfer its own pidfd before readiness deadline: %v", err)
	}
	messages, controlErr := unix.ParseSocketControlMessage(control[:controlN])
	var fds []int
	for _, message := range messages {
		received, rightsErr := unix.ParseUnixRights(&message)
		for _, fd := range received {
			unix.CloseOnExec(fd)
			t.Cleanup(func() { _ = unix.Close(fd) })
		}
		fds = append(fds, received...)
		controlErr = errors.Join(controlErr, rightsErr)
	}
	if controlErr != nil || len(messages) != 1 {
		t.Fatalf("fake npm pidfd control messages = %d: %v", len(messages), controlErr)
	}
	// Even a malformed or duplicate handoff never authorizes signaling a
	// numeric PID. The only received handles originated in PidfdOpen(self).
	role := string(body[:n])
	if len(fds) != 1 || flags&(unix.MSG_TRUNC|unix.MSG_CTRUNC) != 0 || (role != "leader" && role != "child") {
		t.Fatalf("invalid fake npm pidfd handoff: role=%q handles=%d flags=%d", role, len(fds), flags)
	}
	fd := fds[0]
	t.Cleanup(func() { _ = unix.PidfdSendSignal(fd, unix.SIGKILL, nil, 0) })
	if lockedNPMPIDFDExited(t, fd) {
		t.Fatalf("fake npm %s exited before readiness", role)
	}
	return role, fd
}

func sendLockedNPMFixturePIDFD(role string) error {
	fd, err := unix.PidfdOpen(os.Getpid(), 0)
	if err != nil {
		return err
	}
	defer func() { _ = unix.Close(fd) }()
	socket, err := unix.Socket(unix.AF_UNIX, unix.SOCK_DGRAM|unix.SOCK_CLOEXEC, 0)
	if err != nil {
		return err
	}
	defer func() { _ = unix.Close(socket) }()
	address := &unix.SockaddrUnix{Name: os.Getenv("UAP_LOCKED_NPM_FIXTURE_SOCKET")}
	return unix.Sendmsg(socket, []byte(role), unix.UnixRights(fd), address, 0)
}

func lockedNPMPIDFDExited(t *testing.T, fd int) bool {
	t.Helper()
	fds := []unix.PollFd{{Fd: int32(fd), Events: unix.POLLIN}}
	if _, err := unix.Poll(fds, 0); err != nil {
		t.Fatal(err)
	}
	return fds[0].Revents&unix.POLLIN != 0
}

func TestLockedNPMProcessFixture(t *testing.T) {
	root := os.Getenv("UAP_LOCKED_NPM_FIXTURE_ROOT")
	if root == "" {
		return
	}
	if os.Getenv("UAP_LOCKED_NPM_FIXTURE_CHILD") == "1" {
		if err := sendLockedNPMFixturePIDFD("child"); err != nil {
			_, _ = fmt.Fprintln(os.Stderr, "send child pidfd:", err)
			os.Exit(81)
		}
		for {
			if _, err := os.Stat(filepath.Join(root, "mutate")); err == nil {
				_ = os.WriteFile(filepath.Join(os.Getenv("npm_config_cache"), "late-mutation"), []byte("late"), 0o600)
				os.Exit(0)
			}
			time.Sleep(5 * time.Millisecond)
		}
	}
	if err := validateLockedNPMProcessFixture(); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(82)
	}
	_, _ = fmt.Fprint(os.Stdout, strings.Repeat("discard this npm progress output", 100000))
	switch os.Getenv("UAP_LOCKED_NPM_FIXTURE_MODE") {
	case "success":
		entrypoint := filepath.Join("node_modules", "@example", "mcp", "cli.js")
		if err := os.MkdirAll(filepath.Dir(entrypoint), 0o700); err != nil {
			os.Exit(83)
		}
		if err := os.MkdirAll(filepath.Join("node_modules", ".bin"), 0o700); err != nil {
			os.Exit(84)
		}
		if err := os.WriteFile(entrypoint, []byte("// fixture"), 0o600); err != nil {
			os.Exit(85)
		}
		os.Exit(0)
	case "failure":
		_, _ = fmt.Fprint(os.Stderr, strings.Repeat("prefix", 10000)+"final npm diagnostic")
		os.Exit(23)
	}
	executable, err := os.Executable()
	if err != nil {
		os.Exit(86)
	}
	child := exec.CommandContext(context.Background(), executable, "-test.run=^TestLockedNPMProcessFixture$")
	child.Env = append(os.Environ(), "UAP_LOCKED_NPM_FIXTURE_CHILD=1")
	child.Stdout, child.Stderr = os.Stdout, os.Stderr
	if err := child.Start(); err != nil {
		os.Exit(87)
	}
	if err := sendLockedNPMFixturePIDFD("leader"); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, "send leader pidfd:", err)
		_ = child.Process.Kill()
		_ = child.Wait()
		os.Exit(88)
	}
	_ = child.Wait()
	os.Exit(0)
}

func validateLockedNPMProcessFixture() error {
	var args []string
	for index, arg := range os.Args {
		if arg == "--" {
			args = os.Args[index+1:]
			break
		}
	}
	want := "ci --ignore-scripts --omit=dev"
	if os.Getenv("UAP_LOCKED_NPM_EXPECT_OMIT_OPTIONAL") == "1" {
		want += " --omit=optional"
	}
	want += " --no-audit --no-fund"
	if strings.Join(args, " ") != want {
		return fmt.Errorf("unexpected npm command: %v", args)
	}
	if os.Getenv("npm_config_proxy") != "" || os.Getenv("NODE_OPTIONS") != "" ||
		os.Getenv("npm_config_ignore_scripts") != "true" ||
		os.Getenv("npm_config_registry") != "https://registry.npmjs.org/" {
		return fmt.Errorf("npm command did not receive its isolated environment")
	}
	return nil
}
