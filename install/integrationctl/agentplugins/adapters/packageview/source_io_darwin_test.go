//go:build darwin && arm64

package packageview

import (
	"context"
	"errors"
	"runtime"
	"testing"
)

func TestDarwinPolicyFaults(t *testing.T) {
	for _, fault := range []string{"get", "set", "readback", "restore", "restore-readback", "work", "panic", "success"} {
		t.Run(fault, func(t *testing.T) {
			policy := 97
			gets, sets, worked, aborted := 0, 0, 0, 0
			get := func() (int, error) {
				gets++
				if fault == "get" && gets == 1 || fault == "restore-readback" && gets == 3 {
					return 0, errors.New("injected")
				}
				if fault == "readback" && gets == 2 {
					return 98, nil
				}
				return policy, nil
			}
			set := func(n int) error {
				sets++
				if fault == "set" && sets == 1 || fault == "restore" && sets == 2 {
					return errors.New("injected")
				}
				policy = n
				return nil
			}
			var e error
			var panicked any
			func() {
				defer func() { panicked = recover() }()
				e = darwinIO(func() error {
					worked++
					if fault == "panic" {
						panic("test panic")
					}
					if fault == "work" {
						return fail("canceled")
					}
					return nil
				}, func() { aborted++ }, get, set)
			}()
			if fault == "panic" {
				if panicked != "test panic" || aborted == 0 {
					t.Fatal("panic cleanup", panicked, aborted)
				}
			} else if panicked != nil {
				t.Fatal(panicked)
			}
			if fault == "success" {
				if e != nil || worked != 1 || aborted != 0 {
					t.Fatal(e, worked, aborted)
				}
			} else if fault != "panic" && (e == nil || aborted == 0) {
				t.Fatal("failure not propagated", e, aborted)
			}
			if (fault == "get" || fault == "set" || fault == "readback") && worked != 0 {
				t.Fatal("source ran before policy setup")
			}
			if fault != "restore" && policy != 97 {
				t.Fatal("policy not restored", policy)
			}
		})
	}
}
func TestDarwinRealThreadPolicy(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	before, e := materializationGet()
	if e != nil {
		t.Fatalf("UNPROVEN real libc policy prerequisite: %v", e)
	}
	for _, mode := range []string{"success", "error", "cancel", "panic"} {
		var caught any
		func() {
			defer func() { caught = recover() }()
			e = sourceIO(func() error {
				now, e := materializationGet()
				if e != nil || now != materializationOff {
					return fail("platform_unavailable")
				}
				switch mode {
				case "error":
					return fail("fixture_error")
				case "cancel":
					ctx, cancel := context.WithCancel(context.Background())
					cancel()
					return contextError(ctx)
				case "panic":
					panic("real policy panic")
				}
				return nil
			}, func() {})
		}()
		if mode == "success" && e != nil {
			t.Fatal(e)
		}
		if mode == "panic" && caught != "real policy panic" {
			t.Fatal("panic not relayed", caught)
		}
		if mode != "success" && mode != "panic" && e == nil {
			t.Fatal("failure lost")
		}
		after, e := materializationGet()
		if e != nil || after != before {
			t.Fatal("caller thread policy changed", before, after, e)
		}
	}
}

func TestDarwinNoCGORejects(t *testing.T) {
	if darwinCGO {
		t.Skip("specific no-cgo rejection gate; run CGO_ENABLED=0")
	}
	reached := false
	l, e := (Reader{TempDir: "/must-not-resolve-scratch"}).open(context.Background(), "/must-not-resolve-source", &captureHooks{metadata: func(string) error { reached = true; return nil }})
	var safe *Error
	if l != nil || reached || !errors.As(e, &safe) || safe.Code != "platform_unavailable" {
		t.Fatal("no-cgo accessed source or accepted", e)
	}
}
