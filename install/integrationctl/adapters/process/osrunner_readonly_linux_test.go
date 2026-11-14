//go:build linux

package process

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"golang.org/x/sys/unix"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

type readonlyTestContainment struct {
	attachFn    func(*exec.Cmd) error
	terminateFn func() terminationResult
	exitedFn    func() (bool, error)
	closeFn     func() error
}

func (base *readonlyTestContainment) attach(cmd *exec.Cmd) error {
	if base.attachFn != nil {
		return base.attachFn(cmd)
	}
	return nil
}

func (base *readonlyTestContainment) terminate() terminationResult {
	return base.terminateFn()
}

func (base *readonlyTestContainment) exited() (bool, error) {
	return base.exitedFn()
}

func (base *readonlyTestContainment) close() error {
	return base.closeFn()
}

func readonlyBorrowedFile(t *testing.T) *os.File {
	t.Helper()
	path := filepath.Join(t.TempDir(), "original")
	if err := os.WriteFile(path, []byte("original authority"), 0600); err != nil {
		t.Fatal(err)
	}
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = file.Close() })
	return file
}

func requireReadonlyBorrowStillOpen(t *testing.T, original *os.File) {
	t.Helper()
	body := make([]byte, len("original authority"))
	if n, err := original.ReadAt(body, 0); err != nil || n != len(body) || string(body) != "original authority" {
		t.Fatalf("borrowed original changed or closed: %q, %v", body, err)
	}
}

func TestReadonlyExportLeaseDetachesOnlyOwnedCopiesAndRetainsCloseFailure(t *testing.T) {
	original, unrelated := readonlyBorrowedFile(t), readonlyBorrowedFile(t)
	cmd := exec.CommandContext(t.Context(), "unused")
	cmd.ExtraFiles = []*os.File{unrelated}
	lease := newReadonlyExportLease(cmd)
	childFD, err := lease.export(original)
	if err != nil || childFD != 4 || len(cmd.ExtraFiles) != 2 {
		t.Fatal("export did not retain the existing descriptor index", childFD, err)
	}
	exportFile := cmd.ExtraFiles[1]
	if exportFile == original || exportFile.Fd() == original.Fd() {
		t.Fatal("lease adopted its borrowed original")
	}
	flags, err := unix.FcntlInt(exportFile.Fd(), unix.F_GETFD, 0)
	if err != nil || flags&unix.FD_CLOEXEC == 0 {
		t.Fatal("parent export is not close-on-exec", flags, err)
	}
	closeCause, closes := errors.New("retained export close failure"), 0
	lease.closeFile = func(file *os.File) error {
		closes++
		return errors.Join(file.Close(), closeCause)
	}
	for range 2 {
		if err := lease.release(); !errors.Is(err, closeCause) {
			t.Fatal("release lost its first close failure", err)
		}
	}
	if closes != 1 || len(cmd.ExtraFiles) != 1 || cmd.ExtraFiles[0] != unrelated {
		t.Fatal("release repeated a close or detached another owner", closes, cmd.ExtraFiles)
	}
	if _, err := exportFile.Stat(); !errors.Is(err, os.ErrClosed) {
		t.Fatal("owned parent export remains open", err)
	}
	if _, err := lease.export(original); err == nil {
		t.Fatal("released lease accepted another export")
	}
	requireReadonlyBorrowStillOpen(t, original)
	requireReadonlyBorrowStillOpen(t, unrelated)
}

func TestReadonlyFactoryReleasesPartialPreparationAndPreservesOriginalContext(t *testing.T) {
	for _, cancelDuringPrepare := range []bool{false, true} {
		t.Run(fmt.Sprint(cancelDuringPrepare), func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), time.Second)
			defer cancel()
			original := readonlyBorrowedFile(t)
			prepareCause, baseCause := errors.New("prepare failed"), errors.New("base close failed")
			var exported *os.File
			baseCloses := 0
			attributes := &syscall.SysProcAttr{Setpgid: true, UseCgroupFD: true, CgroupFD: 97}
			pipes := duplexPipeFactory{containment: func(cmd *exec.Cmd) (duplexCommandContainment, error) {
				cmd.SysProcAttr = attributes
				return &readonlyTestContainment{closeFn: func() error {
					baseCloses++
					if len(cmd.ExtraFiles) != 0 || cmd.SysProcAttr != attributes {
						t.Fatal("base close saw owned exports or replaced cgroup attributes")
					}
					return baseCause
				}}, nil
			}}
			factory := readonlyDuplexFactory(ctx, pipes, func(got context.Context, cmd *exec.Cmd) (*readonlyExportLease, error) {
				if got != ctx || cmd.SysProcAttr != attributes {
					t.Fatal("preparation lost the original context or containment attributes")
				}
				lease := newReadonlyExportLease(cmd)
				if _, err := lease.export(original); err != nil {
					t.Fatal(err)
				}
				exported = cmd.ExtraFiles[0]
				if cancelDuringPrepare {
					cancel()
					return lease, nil
				}
				return lease, prepareCause
			})
			containment, err := factory.containment(exec.CommandContext(t.Context(), "unused"))
			want := prepareCause
			if cancelDuringPrepare {
				want = context.Canceled
			}
			if containment != nil || !errors.Is(err, want) || !errors.Is(err, baseCause) || baseCloses != 1 {
				t.Fatal("construction lost original failures or containment ownership", containment, err, baseCloses)
			}
			if _, err := exported.Stat(); !errors.Is(err, os.ErrClosed) {
				t.Fatal("partial preparation retained its export", err)
			}
			requireReadonlyBorrowStillOpen(t, original)
		})
	}
}

func TestReadonlyFactoryCancellationAfterBaseAllocationPreventsPreparation(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	baseCloses := 0
	pipes := duplexPipeFactory{containment: func(*exec.Cmd) (duplexCommandContainment, error) {
		cancel()
		return &readonlyTestContainment{closeFn: func() error { baseCloses++; return nil }}, nil
	}}
	factory := readonlyDuplexFactory(ctx, pipes, func(context.Context, *exec.Cmd) (*readonlyExportLease, error) {
		t.Fatal("canceled context reached view preparation")
		return nil, nil
	})
	containment, err := factory.containment(exec.CommandContext(t.Context(), "unused"))
	if containment != nil || !errors.Is(err, context.Canceled) || baseCloses != 1 {
		t.Fatal("cancellation lost the allocated base owner", containment, err, baseCloses)
	}
}

func TestReadonlyFactoryRefusesMissingReleasedOrForeignExportSetsBeforeStart(t *testing.T) {
	for _, fault := range []string{"missing", "released", "foreign", "wrong-command"} {
		t.Run(fault, func(t *testing.T) {
			original, foreign := readonlyBorrowedFile(t), readonlyBorrowedFile(t)
			baseCloses := 0
			pipes := duplexPipeFactory{containment: func(*exec.Cmd) (duplexCommandContainment, error) {
				return &readonlyTestContainment{closeFn: func() error { baseCloses++; return nil }}, nil
			}}
			factory := readonlyDuplexFactory(t.Context(), pipes, func(_ context.Context, cmd *exec.Cmd) (*readonlyExportLease, error) {
				if fault == "missing" {
					return nil, nil
				}
				if fault == "wrong-command" {
					cmd = exec.CommandContext(t.Context(), "different")
				}
				lease := newReadonlyExportLease(cmd)
				if _, err := lease.export(original); err != nil {
					t.Fatal(err)
				}
				if fault == "released" {
					if err := lease.release(); err != nil {
						t.Fatal(err)
					}
				}
				if fault == "foreign" {
					cmd.ExtraFiles = append(cmd.ExtraFiles, foreign)
				}
				return lease, nil
			})
			cmd := exec.CommandContext(t.Context(), "unused")
			containment, err := factory.containment(cmd)
			if containment != nil || err == nil || baseCloses != 1 || cmd.Process != nil {
				t.Fatal("unowned export set reached execution", containment, err, baseCloses)
			}
			requireReadonlyBorrowStillOpen(t, original)
			requireReadonlyBorrowStillOpen(t, foreign)
		})
	}
}

func TestReadonlyExportLeaseBoundAndClosedBorrowRefuseWithoutAdoption(t *testing.T) {
	original := readonlyBorrowedFile(t)
	lease := newReadonlyExportLease(exec.CommandContext(t.Context(), "unused"))
	t.Cleanup(func() { _ = lease.release() })
	for i := range readonlyExportLimit {
		fd, err := lease.export(original)
		if err != nil || fd != 3+i {
			t.Fatal("finite export sequence differs", fd, err)
		}
	}
	if _, err := lease.export(original); err == nil || len(lease.files) != readonlyExportLimit {
		t.Fatal("export bound did not refuse before duplicating", err)
	}
	if err := lease.release(); err != nil {
		t.Fatal(err)
	}
	requireReadonlyBorrowStillOpen(t, original)
	closed := readonlyBorrowedFile(t)
	if err := closed.Close(); err != nil {
		t.Fatal(err)
	}
	empty := newReadonlyExportLease(exec.CommandContext(t.Context(), "unused"))
	if _, err := empty.export(closed); err == nil || len(empty.files) != 0 || len(empty.cmd.ExtraFiles) != 0 {
		t.Fatal("closed borrow was adopted", err)
	}
}

func TestReadonlyContainmentAttachFailureAndCloseRetryKeepExportOwnership(t *testing.T) {
	original := readonlyBorrowedFile(t)
	cmd := exec.CommandContext(t.Context(), "unused")
	lease := newReadonlyExportLease(cmd)
	if _, err := lease.export(original); err != nil {
		t.Fatal(err)
	}
	exported := cmd.ExtraFiles[0]
	attachCause, leaseCause, baseCause := errors.New("attach failed"), errors.New("export close failed"), errors.New("base close failed")
	closes, baseCloses := 0, 0
	lease.closeFile = func(file *os.File) error { closes++; return errors.Join(file.Close(), leaseCause) }
	base := &readonlyTestContainment{
		attachFn: func(got *exec.Cmd) error {
			if got != cmd || len(cmd.ExtraFiles) != 1 {
				t.Fatal("attach did not receive its live export")
			}
			return attachCause
		},
		closeFn: func() error {
			baseCloses++
			if len(cmd.ExtraFiles) != 0 {
				t.Fatal("retry could close a stale export through the base's generic loop")
			}
			if baseCloses == 1 {
				return baseCause
			}
			return nil
		},
	}
	containment := &readonlyDuplexContainment{base: base, lease: lease}
	if err := containment.attach(cmd); !errors.Is(err, attachCause) || !errors.Is(err, leaseCause) {
		t.Fatal("attach lost its base or release error", err)
	}
	if err := containment.close(); !errors.Is(err, leaseCause) || !errors.Is(err, baseCause) {
		t.Fatal("first close lost its base or release error", err)
	}
	if err := containment.close(); !errors.Is(err, leaseCause) || errors.Is(err, baseCause) {
		t.Fatal("retry did not retain the one-time lease error", err)
	}
	if closes != 1 || baseCloses != 2 {
		t.Fatal("close retried the wrong owner", closes, baseCloses)
	}
	if _, err := exported.Stat(); !errors.Is(err, os.ErrClosed) {
		t.Fatal("attach failure retained its export", err)
	}
	requireReadonlyBorrowStillOpen(t, original)
}

func TestReadonlyFactoryInputAndStartFailuresUseUnchangedDuplexCleanup(t *testing.T) {
	for _, inputFailure := range []bool{false, true} {
		t.Run(fmt.Sprint(inputFailure), func(t *testing.T) {
			original := readonlyBorrowedFile(t)
			var exported *os.File
			baseCloses := 0
			inputCause := errors.New("input pipe failed")
			pipes := defaultDuplexPipeFactory
			pipes.containment = func(*exec.Cmd) (duplexCommandContainment, error) {
				return &readonlyTestContainment{closeFn: func() error { baseCloses++; return nil }}, nil
			}
			if inputFailure {
				pipes.input = func(*exec.Cmd) (io.WriteCloser, error) { return nil, inputCause }
			}
			factory := readonlyDuplexFactory(t.Context(), pipes, func(_ context.Context, cmd *exec.Cmd) (*readonlyExportLease, error) {
				lease := newReadonlyExportLease(cmd)
				if _, err := lease.export(original); err != nil {
					t.Fatal(err)
				}
				exported = cmd.ExtraFiles[0]
				return lease, nil
			})
			err := runDuplexMode(t.Context(), ports.Command{Argv: []string{filepath.Join(t.TempDir(), "missing-executable")}}, func(io.Writer, io.Reader) error {
				return errors.New("pre-start refusal reached exchange")
			}, factory, true)
			if err == nil || baseCloses != 1 || (inputFailure && !errors.Is(err, inputCause)) || (!inputFailure && !errors.Is(err, os.ErrNotExist)) {
				t.Fatal("unchanged runner lost the pre-start failure", err, baseCloses)
			}
			if _, err := exported.Stat(); !errors.Is(err, os.ErrClosed) {
				t.Fatal("pre-start refusal retained export", err)
			}
			requireReadonlyBorrowStillOpen(t, original)
		})
	}
}

func TestReadonlyFactoryPreservesPlannedDuplexExchangeAndSoleReap(t *testing.T) {
	if os.Getenv("AGENTPLUGINS_READONLY_LEASE_HELPER") == "1" {
		line, err := bufio.NewReader(os.Stdin).ReadString('\n')
		if err != nil {
			os.Exit(9)
		}
		_, _ = fmt.Fprint(os.Stdout, line)
		_, _ = io.Copy(io.Discard, os.Stdin)
		os.Exit(0)
	}
	for _, cancelOnClose := range []bool{false, true} {
		t.Run(fmt.Sprint(cancelOnClose), func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
			defer cancel()
			original := readonlyBorrowedFile(t)
			var selected *exec.Cmd
			var exported *os.File
			closes, terminations := 0, 0
			pipes := defaultDuplexPipeFactory
			pipes.containment = func(cmd *exec.Cmd) (duplexCommandContainment, error) {
				selected = cmd
				observer := &commandContainment{cmd: cmd}
				return &readonlyTestContainment{
					exitedFn: observer.exited,
					terminateFn: func() terminationResult {
						terminations++
						killErr := cmd.Process.Kill()
						if killErr != nil {
							return terminationResult{err: killErr}
						}
						var status unix.Siginfo
						var waitErr error
						deadline := time.Now().Add(time.Second)
						for time.Now().Before(deadline) {
							waitErr = unix.Waitid(unix.P_PID, cmd.Process.Pid, &status, unix.WEXITED|unix.WNOWAIT|unix.WNOHANG, nil)
							if waitErr != nil || status.Signo != 0 {
								break
							}
							time.Sleep(time.Millisecond)
						}
						return terminationResult{leaderStopped: waitErr == nil && status.Signo != 0,
							leaderTerminationInitiated: killErr == nil, err: errors.Join(killErr, waitErr)}
					},
					closeFn: func() error {
						closes++
						if len(cmd.ExtraFiles) != 0 || cmd.ProcessState != nil {
							return errors.New("close saw stale exports or a previously reaped child")
						}
						if cancelOnClose {
							cancel()
						}
						return nil
					},
				}, nil
			}
			factory := readonlyDuplexFactory(ctx, pipes, func(_ context.Context, cmd *exec.Cmd) (*readonlyExportLease, error) {
				lease := newReadonlyExportLease(cmd)
				if _, err := lease.export(original); err != nil {
					t.Fatal(err)
				}
				exported = cmd.ExtraFiles[0]
				return lease, nil
			})
			command := ports.Command{Argv: []string{os.Args[0], "-test.run=^TestReadonlyFactoryPreservesPlannedDuplexExchangeAndSoleReap$"},
				Env: []string{"AGENTPLUGINS_READONLY_LEASE_HELPER=1"}}
			err := runDuplexMode(ctx, command, func(stdin io.Writer, stdout io.Reader) error {
				if _, err := io.WriteString(stdin, "verified\n"); err != nil {
					return err
				}
				line, err := bufio.NewReader(stdout).ReadString('\n')
				if err != nil || line != "verified\n" {
					return fmt.Errorf("inert response differs: %q: %w", line, err)
				}
				return nil
			}, factory, true)
			if cancelOnClose && !errors.Is(err, context.Canceled) || !cancelOnClose && err != nil {
				t.Fatal("exchange or late cancellation changed", err)
			}
			if terminations != 1 || closes != 1 || selected.ProcessState == nil || !strings.Contains(selected.ProcessState.String(), "killed") {
				t.Fatal("original planned termination/sole reap path differs", terminations, closes, selected.ProcessState)
			}
			if _, err := exported.Stat(); !errors.Is(err, os.ErrClosed) {
				t.Fatal("successful attach retained its parent export", err)
			}
			requireReadonlyBorrowStillOpen(t, original)
		})
	}
}
