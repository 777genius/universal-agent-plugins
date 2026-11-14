//go:build linux || darwin

package pluginkitai_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"testing"
	"time"

	pluginkitai "github.com/777genius/plugin-kit-ai/sdk"
)

// Red: a shared SDK name syscall constant is unavailable on Linux/386, breaking
// historical pipe users before they can invoke any public SDK API.
func TestCursorObserverLinux386Build(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, filepath.Join(runtime.GOROOT(), "bin/go"), "build", "-p=2", ".", "./cursor")
	cmd.Env = append(os.Environ(), "GOOS=linux", "GOARCH=386", "CGO_ENABLED=0")
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("Linux/386 public SDK build: %v\n%s", err, output)
	}
}

// Node/libuv's anonymous stream socketpairs are inherited as real stdio, not
// injected IO. Red on the pipe-only public SDK: no callback, no response, exit 1.
// The existing independent consumer reports joined callbacks and closed owned
// descriptors immediately after RunCursorObserver, before process exit.
func TestCursorObserverSocketProcess(t *testing.T) {
	binary := buildCursorConsumer(t)
	for _, mode := range []string{"normal", "owned", "cooperative", "cancel-reading"} {
		t.Run(mode, func(t *testing.T) {
			input := cursorInput
			calls := 1
			if mode == "cancel-reading" {
				input, calls = "", 0 // Keep the peer open; cancellation must wake read.
			}
			code, out, records := runCursorSocketConsumer(t, binary, mode, input, "normal")
			if code != 0 || out != "{}\n" || len(records) != calls+1 || !records[len(records)-1].Clean || records[len(records)-1].Code != 0 {
				t.Fatalf("exit=%d stdout=%q records=%+v", code, out, records)
			}
			if calls == 1 && (records[0].Kind != "cursor" || records[0].Event.ConversationID != "TEST_CONVERSATION") {
				t.Fatal("missing native stop callback")
			}
		})
	}
}

// Red if socket writes escape the output deadline, SIGPIPE kills the child,
// or a failed response abandons owned descriptors/callbacks. Pipe tests cannot
// establish these properties for socket send buffers and peer shutdown.
func TestCursorObserverSocketOutput(t *testing.T) {
	binary := buildCursorConsumer(t)
	for _, output := range []string{"broken", "full"} {
		t.Run(output, func(t *testing.T) {
			code, _, records := runCursorSocketConsumer(t, binary, "normal", cursorInput, output)
			if code != 1 || len(records) != 2 || records[0].Kind != "cursor" || !records[1].Clean || records[1].Code != 1 {
				t.Fatalf("output=%s exit=%d records=%+v", output, code, records)
			}
		})
	}
}

func cursorSocketPair(t *testing.T, kind int) (*os.File, *os.File) {
	t.Helper()
	fds, err := syscall.Socketpair(syscall.AF_UNIX, kind, 0)
	if err != nil {
		t.Fatal(err)
	}
	for _, fd := range fds {
		syscall.CloseOnExec(fd)
	}
	// The child's endpoint stays blocking, as with a plain inherited NewFile.
	if err := syscall.SetNonblock(fds[1], true); err != nil {
		_ = syscall.Close(fds[0])
		_ = syscall.Close(fds[1])
		t.Fatal(err)
	}
	child := os.NewFile(uintptr(fds[0]), "TEST-child-socket")
	peer := os.NewFile(uintptr(fds[1]), "TEST-peer-socket")
	t.Cleanup(func() { _ = child.Close(); _ = peer.Close() })
	if err := peer.SetDeadline(time.Now().Add(2 * time.Second)); err != nil {
		t.Fatal(err)
	}
	return child, peer
}

func cursorSocketControl(t *testing.T, file *os.File, action func(int) error) {
	t.Helper()
	raw, err := file.SyscallConn()
	if err != nil {
		t.Fatal(err)
	}
	var actionErr error
	if err := raw.Control(func(fd uintptr) { actionErr = action(int(fd)) }); err != nil {
		t.Fatal(err)
	}
	if actionErr != nil {
		t.Fatal(actionErr)
	}
}

func fillCursorSocket(t *testing.T, file *os.File) {
	t.Helper()
	cursorSocketControl(t, file, func(fd int) error {
		if err := syscall.SetNonblock(fd, true); err != nil {
			return err
		}
		buf := make([]byte, 64<<10)
		for total := 0; total < 4<<20; {
			n, err := syscall.Write(fd, buf)
			if errors.Is(err, syscall.EAGAIN) && total > 0 {
				return syscall.SetNonblock(fd, false)
			}
			if err != nil {
				return err
			}
			total += n
		}
		return errors.New("TEST socket send buffer did not fill within bound")
	})
}

func runCursorSocketConsumer(t *testing.T, binary, mode, input, output string) (int, string, []cursorRecord) {
	t.Helper()
	in, inputPeer := cursorSocketPair(t, syscall.SOCK_STREAM)
	out, outputPeer := cursorSocketPair(t, syscall.SOCK_STREAM)
	diag, diagPeer := cursorSocketPair(t, syscall.SOCK_STREAM)
	switch output {
	case "broken":
		if err := outputPeer.Close(); err != nil {
			t.Fatal(err)
		}
	case "full":
		fillCursorSocket(t, out)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, binary, "CursorStop", mode)
	cmd.Env = append(os.Environ(), "GORACE=atexit_sleep_ms=0")
	cmd.Stdin, cmd.Stdout, cmd.Stderr = in, out, diag
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	waited := false
	defer func() {
		if !waited {
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
		}
	}()
	_ = in.Close()
	_ = out.Close()
	_ = diag.Close()
	start := time.Now()
	if input != "" {
		if _, err := io.Copy(inputPeer, bytes.NewBufferString(input)); err != nil {
			t.Fatal(err)
		}
		cursorSocketControl(t, inputPeer, func(fd int) error { return syscall.Shutdown(fd, syscall.SHUT_WR) })
	}
	err := cmd.Wait()
	waited = true
	code := 0
	if err != nil {
		var exit *exec.ExitError
		if !errors.As(err, &exit) {
			t.Fatal(err)
		}
		code = exit.ExitCode()
	}
	if ctx.Err() != nil || time.Since(start) > time.Second {
		t.Fatalf("socket child exceeded bounded return: %v", err)
	}
	b, err := io.ReadAll(diagPeer)
	if err != nil {
		t.Fatal(err)
	}
	var response []byte
	if output == "normal" {
		response, err = io.ReadAll(outputPeer)
		if err != nil {
			t.Fatal(err)
		}
	}
	return code, string(response), cursorRecords(t, string(b))
}

// Red if arbitrary ModeSocket is admitted, either named endpoint is overlooked,
// or a regular file is duplicated as supported IPC. A canceled context
// distinguishes an immediate preparation refusal from adapted IO cancellation:
// rejected descriptors must return their setup error before attempting any IO.
func TestCursorObserverUnsupportedDescriptors(t *testing.T) {
	factories := map[string]func(*testing.T) *os.File{
		"regular file": func(t *testing.T) *os.File {
			f, err := os.CreateTemp(t.TempDir(), "TEST-file")
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = f.Close() })
			return f
		},
		"UNIX datagram pair": func(t *testing.T) *os.File {
			f, _ := cursorSocketPair(t, syscall.SOCK_DGRAM)
			return f
		},
		"UDP": func(t *testing.T) *os.File { return cursorTestSocket(t, syscall.AF_INET, syscall.SOCK_DGRAM) },
		"named local UNIX stream": func(t *testing.T) *os.File {
			_, server := cursorNamedSocketPair(t, false)
			return server
		},
		"named peer UNIX stream": func(t *testing.T) *os.File {
			client, _ := cursorNamedSocketPair(t, false)
			return client
		},
		"connected TCP": func(t *testing.T) *os.File {
			client, _ := cursorTCPSocketPair(t)
			return client
		},
	}
	if runtime.GOOS == "linux" {
		for _, server := range []bool{false, true} {
			name := "abstract peer UNIX stream"
			if server {
				name = "abstract local UNIX stream"
			}
			factories[name] = func(t *testing.T) *os.File {
				client, accepted := cursorNamedSocketPair(t, true)
				if server {
					return accepted
				}
				return client
			}
			nulBytes := 32
			factories["all-NUL "+name] = func(t *testing.T) *os.File {
				named, peer := cursorSocketPair(t, syscall.SOCK_STREAM)
				// Go's textual name is indistinguishable from an unnamed pair;
				// the returned native socklen must reject this abstract address.
				cursorSocketControl(t, named, func(fd int) error {
					// All-NUL names share a finite kernel namespace. Reserve a free
					// length without interfering with another TEST owner's sockets.
					for nulBytes < syscall.SizeofSockaddrUnix-3 {
						nulBytes++
						err := syscall.Bind(fd, &syscall.SockaddrUnix{Name: "@" + strings.Repeat("\x00", nulBytes)})
						if !errors.Is(err, syscall.EADDRINUSE) {
							return err
						}
					}
					return errors.New("TEST all-NUL address namespace exhausted")
				})
				if server {
					return named
				}
				return peer
			}
		}
	}
	for name, create := range factories {
		t.Run(name, func(t *testing.T) {
			in, out := create(t), create(t)
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			owned := pluginkitai.NewCursorObserverPipeIO(in, out)
			_, readErr := owned.ReadStdin(ctx)
			writeErr := owned.WriteStdoutContext(ctx, []byte("{}\n"))
			app := pluginkitai.New(pluginkitai.Config{Args: []string{"TEST", "CursorStop"}, IO: owned})
			code := app.RunCursorObserver(ctx) // Also joins/closes any prepared IO.
			if readErr == nil || writeErr == nil || errors.Is(readErr, context.Canceled) || errors.Is(writeErr, context.Canceled) || code != 1 {
				t.Fatalf("unsupported descriptor admitted: read=%v write=%v exit=%d", readErr, writeErr, code)
			}
			for _, f := range []*os.File{in, out} {
				if _, err := f.Stat(); !errors.Is(err, os.ErrClosed) {
					t.Fatalf("owned unsupported original remains open: %v", err)
				}
			}
		})
	}
}

func cursorTestSocket(t *testing.T, domain, kind int) *os.File {
	t.Helper()
	fd, err := syscall.Socket(domain, kind, 0)
	if err != nil {
		t.Fatal(err)
	}
	return cursorTestSocketFile(t, fd)
}

func cursorTestSocketFile(t *testing.T, fd int) *os.File {
	t.Helper()
	syscall.CloseOnExec(fd)
	f := os.NewFile(uintptr(fd), "TEST-unsupported-socket")
	t.Cleanup(func() { _ = f.Close() })
	return f
}

func cursorNamedSocketPair(t *testing.T, abstract bool) (*os.File, *os.File) {
	t.Helper()
	named, peer := cursorSocketPair(t, syscall.SOCK_STREAM)
	dir, err := os.MkdirTemp(os.TempDir(), "TEST-socket-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	path := filepath.Join(dir, "s")
	if abstract {
		// The NUL following the leading NUL hides the remaining unique TEST name
		// in Go's text-only sockaddr wrapper. It must not qualify as anonymous.
		path = "@\x00" + filepath.Base(dir)
	} else {
		cwd, err := os.Getwd()
		if err != nil {
			t.Fatal(err)
		}
		path, err = filepath.Rel(cwd, path)
		if err != nil {
			t.Fatal(err)
		}
	}
	cursorSocketControl(t, named, func(fd int) error { return syscall.Bind(fd, &syscall.SockaddrUnix{Name: path}) })
	return peer, named
}

func cursorTCPSocketPair(t *testing.T) (*os.File, *os.File) {
	t.Helper()
	listener := cursorTestSocket(t, syscall.AF_INET, syscall.SOCK_STREAM)
	var address syscall.Sockaddr = &syscall.SockaddrInet4{Addr: [4]byte{127, 0, 0, 1}}
	cursorSocketControl(t, listener, func(fd int) error {
		if err := syscall.Bind(fd, address); err != nil {
			return err
		}
		if err := syscall.Listen(fd, 1); err != nil {
			return err
		}
		var err error
		address, err = syscall.Getsockname(fd)
		return err
	})
	client := cursorTestSocket(t, syscall.AF_INET, syscall.SOCK_STREAM)
	cursorSocketControl(t, client, func(fd int) error { return syscall.Connect(fd, address) })
	var accepted int
	cursorSocketControl(t, listener, func(fd int) error {
		var err error
		accepted, _, err = syscall.Accept(fd)
		return err
	})
	return client, cursorTestSocketFile(t, accepted)
}
