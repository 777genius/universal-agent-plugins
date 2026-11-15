//go:build linux

package grok

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

func grokTailPipe(t *testing.T) (*os.File, *os.File) {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = r.Close(); _ = w.Close() })
	return r, w
}

func grokTailFlags(t *testing.T, file *os.File) int {
	t.Helper()
	raw, err := file.SyscallConn()
	if err != nil {
		t.Fatal(err)
	}
	flags := 0
	var flagErr error
	err = raw.Control(func(fd uintptr) { flags, flagErr = unix.FcntlInt(fd, unix.F_GETFL, 0) })
	if err != nil || flagErr != nil {
		t.Fatal(errors.Join(err, flagErr))
	}
	return flags
}

func assertGrokTailClosed(t *testing.T, fd int) {
	t.Helper()
	if _, err := unix.FcntlInt(uintptr(fd), unix.F_GETFD, 0); !errors.Is(err, unix.EBADF) {
		t.Fatalf("retained fd %d not closed: %v", fd, err)
	}
}

func TestEnablementACPTailActualPipeDoesNotCompeteOrChangeFlags(t *testing.T) {
	r, w := grokTailPipe(t)
	before := grokTailFlags(t, r)
	tail, err := captureGrokACPTail(r)
	if err != nil {
		t.Fatal(err)
	}
	fd := tail.fd
	defer func() { _ = tail.close() }()
	if got := grokTailFlags(t, r); got != before {
		t.Fatalf("capture changed pipe flags: %x -> %x", before, got)
	}
	flags, err := unix.FcntlInt(uintptr(fd), unix.F_GETFD, 0)
	if err != nil || flags&unix.FD_CLOEXEC == 0 {
		t.Fatalf("duplicate lacks CLOEXEC: %x %v", flags, err)
	}
	if _, err := w.Write([]byte("response\n")); err != nil {
		t.Fatal(err)
	}
	reader := bufio.NewReader(r)
	if line, err := reader.ReadString('\n'); line != "response\n" || err != nil {
		t.Fatalf("original lost its response: %q %v", line, err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	if err := r.Close(); err != nil {
		t.Fatal(err)
	}
	if err := tail.drain(context.Background(), nil); err != nil {
		t.Fatal(err)
	}
	assertGrokTailClosed(t, fd)
}

func TestEnablementACPTailPrefetchedAndPartialFramesRefuse(t *testing.T) {
	for _, prefetched := range []bool{true, false} {
		t.Run(map[bool]string{true: "prefetched", false: "delayed-partial"}[prefetched], func(t *testing.T) {
			r, w := grokTailPipe(t)
			tail, err := captureGrokACPTail(r)
			if err != nil {
				t.Fatal(err)
			}
			fd := tail.fd
			defer func() { _ = tail.close() }()
			body := "response\n"
			if prefetched {
				body += `{"jsonrpc":`
			}
			if _, err := io.WriteString(w, body); err != nil {
				t.Fatal(err)
			}
			reader := bufio.NewReader(r)
			if _, err := reader.ReadString('\n'); err != nil {
				t.Fatal(err)
			}
			pending, err := reader.Peek(reader.Buffered())
			if err != nil {
				t.Fatal(err)
			}
			pending = bytes.Clone(pending)
			if prefetched && len(pending) == 0 {
				t.Fatal("fixture did not prefetch trailing bytes")
			}
			if !prefetched {
				if _, err := io.WriteString(w, `{"jsonrpc":`); err != nil {
					t.Fatal(err)
				}
			}
			_ = w.Close()
			_ = r.Close()
			if err := tail.drain(context.Background(), pending); err == nil {
				t.Fatal("trailing partial frame accepted")
			}
			assertGrokTailClosed(t, fd)
		})
	}
}

func TestEnablementACPTailCancellationAndUnclosedWriterRefuse(t *testing.T) {
	for _, canceled := range []bool{true, false} {
		t.Run(map[bool]string{true: "canceled", false: "unclosed-writer"}[canceled], func(t *testing.T) {
			r, _ := grokTailPipe(t)
			tail, err := captureGrokACPTail(r)
			if err != nil {
				t.Fatal(err)
			}
			fd := tail.fd
			_ = r.Close()
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
			defer cancel()
			if canceled {
				cancel()
			}
			start := time.Now()
			if err := tail.drain(ctx, nil); err == nil {
				t.Fatal("unsettled pipe accepted")
			}
			if time.Since(start) > time.Second {
				t.Fatal("final EOF was not bounded")
			}
			assertGrokTailClosed(t, fd)
		})
	}
}

func TestEnablementACPTailOverflowDupFailureAndEarlyCleanup(t *testing.T) {
	r, w := grokTailPipe(t)
	_, err := duplicateGrokACPPipe(r, func(int) (int, error) { return -1, unix.EMFILE })
	if !errors.Is(err, unix.EMFILE) {
		t.Fatalf("dup failure hidden: %v", err)
	}
	if _, err := captureGrokACPTail(strings.NewReader("")); err == nil {
		t.Fatal("non-pipe accepted")
	}
	tail, err := captureGrokACPTail(r)
	if err != nil {
		t.Fatal(err)
	}
	fd := tail.fd
	if err := tail.close(); err != nil {
		t.Fatal(err)
	}
	if err := tail.close(); err != nil {
		t.Fatal(err)
	}
	assertGrokTailClosed(t, fd)
	tail, err = captureGrokACPTail(r)
	if err != nil {
		t.Fatal(err)
	}
	fd = tail.fd
	_ = r.Close()
	_ = w.Close()
	if err := tail.drain(context.Background(), bytes.Repeat([]byte{'x'}, grokACPTailLimit+1)); err == nil {
		t.Fatal("overflow accepted")
	}
	assertGrokTailClosed(t, fd)
}
