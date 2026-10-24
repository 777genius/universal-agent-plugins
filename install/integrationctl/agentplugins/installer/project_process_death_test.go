package installer

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/codex"
)

// The parent kills a real SDK child without unwinding its callbacks. Client
// commands use only the existing inert Codex fixture, never a vendor binary or
// MCP server. Each reopened engine uses the ordinary Codex adapter.
func TestProjectProcessDeathReopenRetainsUnacknowledgedNativeAttempt(t *testing.T) {
	skipWindowsLauncherExecuteBit(t)
	probe := buildCodexProbe(t)
	for _, boundary := range []string{deathManagedBoundary, deathNativeBoundary} {
		t.Run(boundary, func(t *testing.T) {
			base, err := filepath.EvalSymlinks(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			deathWrite(t, filepath.Join(base, "codex-fixture"), deathRead(t, probe))
			if err := os.Chmod(filepath.Join(base, "codex-fixture"), 0700); err != nil {
				t.Fatal(err)
			}
			deathPackage(t, base, deathComponent, deathOldVersion)
			freshPackage := deathPackage(t, base, "fresh-death", deathOldVersion)
			a, b := filepath.Join(base, "A"), filepath.Join(base, "B")
			user := filepath.Join(base, "user", ".codex")
			deathWrite(t, filepath.Join(user, "config.toml"), "# original disposable user profile\n")
			for _, project := range []string{a, b} {
				deathWrite(t, filepath.Join(project, ".codex", "config.toml"), "foreign_setting = true\n")
				deathWrite(t, filepath.Join(project, ".agents", "skills", "foreign", "SKILL.md"), "foreign skill\n")
			}
			var aEngine *Engine
			for _, project := range []string{"", a, b} {
				engine, _ := deathEngine(t, base, project, nil, nil)
				result := deathApply(t, engine, deathRequest(t, engine, base, project, OpInstall))
				if result.Binding.DataRoot == "" {
					t.Fatal("public install omitted owned data authority")
				}
				deathWrite(t, filepath.Join(result.Binding.DataRoot, "keep.txt"), "retained owner data\n")
				if project == a {
					aEngine = engine
				}
			}
			deathWrite(t, filepath.Join(user, "config.toml"), deathRead(t, filepath.Join(user, "config.toml"))+"\n[foreign_fixture]\nkeep = true\n")
			deathWrite(t, filepath.Join(user, "foreign.txt"), "foreign user bytes\n")
			before := deathBinding(t, base, a)
			update := deathRequest(t, aEngine, base, a, OpUpdate)
			oldConfig := deathRead(t, filepath.Join(a, ".codex", "config.toml"))
			userNative, userState, bTree := deathTree(t, user), deathTree(t, filepath.Join(base, "user-uap")), deathTree(t, b)
			deathPackage(t, base, deathComponent, deathNewVersion)
			barrier := deathKillAtBarrier(t, base, boundary, func(observed deathBarrier) {
				expected := deathProveBoundary(t, base, boundary, before, oldConfig)
				if !reflect.DeepEqual(observed, expected) {
					t.Fatalf("child barrier differs from independently read persisted facts: %+v, want %+v", observed, expected)
				}
			})
			if barrier.Boundary != boundary {
				t.Fatal("wrong child boundary")
			}
			deathProveBoundary(t, base, boundary, before, oldConfig)
			retained := deathTree(t, a)
			hostCalls := 0
			fresh, runner := deathEngine(t, base, a, nil, func(context.Context, BindingFacts) error { hostCalls++; return nil })
			view, inspectErr := fresh.Inspect(t.Context())
			const uncertainty = "native attempt has no persisted intent; retain uncertainty"
			if inspectErr == nil || !view.Recovery.Required || view.Recovery.Reason != uncertainty || len(view.Installations) != 1 || len(view.Installations[0].Bindings) != 1 {
				t.Fatalf("fresh read-only inspect lost pending facts: %+v, %v", view, inspectErr)
			}
			deathAssertTree(t, a, retained)
			for _, operation := range []Operation{OpUpdate, OpRepair, OpRemove, OpInstall} {
				req := update
				authority := *update.EnvironmentAuthority
				req.EnvironmentAuthority = &authority
				req.Operation, req.OperationID = operation, "fenced-after-death-"+string(operation)
				authority.ExpectedAction = string(operation)
				if operation == OpInstall {
					req.PackageRoot, req.InstallationID = freshPackage, ""
					authority.Alias, authority.ReceiptID, authority.InstallationID, authority.ClientBindingID = "fresh-death", "", "", ""
				}
				handle, err := fresh.Prepare(t.Context(), req)
				if err == nil {
					result, applyErr := fresh.Apply(t.Context(), handle, Decision{Confirmed: true})
					err = applyErr
					if result.Mutated || result.Outcome != OutcomeRecovery {
						t.Fatalf("ordinary %s was not fenced: %+v, %v", operation, result, err)
					}
				}
				if handle != nil {
					if closeErr := handle.Close(); closeErr != nil {
						t.Fatal(closeErr)
					}
				}
				blocked := errors.Is(err, ErrRecoveryRequired) || err != nil && strings.Contains(err.Error(), "native activation attempt") && strings.Contains(err.Error(), "unresolved")
				if operation == OpRemove && errors.Is(err, ErrInvalidRequest) && strings.Contains(err.Error(), "removal binding scope or native attempt differs") {
					// The exact selected scope/binding was proven above; the real
					// removal preflight refuses its retained native attempt earlier.
					blocked = true
				}
				if !blocked {
					t.Fatalf("ordinary %s refused for an unrelated reason: %v", operation, err)
				}
				deathAssertTree(t, a, retained)
			}
			if result, err := fresh.RecoverEnvironment(t.Context(), view, "project-default"); !errors.Is(err, ErrRecoveryRequired) || result.Outcome != OutcomeRecovery {
				t.Fatalf("zero-selected delivery guessed native recovery: %+v, %v", result, err)
			}
			other, _ := deathEngine(t, base, b, nil, nil)
			if _, err := other.RecoverEnvironment(t.Context(), view, "project-default"); !errors.Is(err, ErrPlanChanged) {
				t.Fatalf("project B accepted project A recovery scope: %v", err)
			}
			again, err := fresh.Inspect(t.Context())
			if err == nil || !reflect.DeepEqual(view, again) || runner.calls != 0 || hostCalls != 0 {
				t.Fatalf("inspect/retry/recovery changed pending facts or called host/native process: %+v, %v; runner=%d host=%d", again, err, runner.calls, hostCalls)
			}
			deathAssertTree(t, a, retained)
			deathAssertTree(t, user, userNative)
			deathAssertTree(t, filepath.Join(base, "user-uap"), userState)
			deathAssertTree(t, b, bTree)
			deathProveBoundary(t, base, boundary, before, oldConfig)
		})
	}
}

func TestProjectProcessDeathChild(t *testing.T) {
	var args []string
	for i, arg := range os.Args {
		if arg == "--" {
			args = os.Args[i+1:]
			break
		}
	}
	if len(args) == 0 {
		t.Skip("disposable parent-controlled SDK child only")
	}
	if len(args) != 2 || args[1] != deathManagedBoundary && args[1] != deathNativeBoundary {
		t.Fatal("invalid process-death fixture arguments")
	}
	base, boundary := args[0], args[1]
	project := filepath.Join(base, "A")
	before := deathBinding(t, base, project)
	oldConfig := deathRead(t, filepath.Join(project, ".codex", "config.toml"))
	var adapter clients.Adapter
	var committed func(context.Context, BindingFacts) error
	if boundary == deathManagedBoundary {
		committed = func(_ context.Context, facts BindingFacts) error {
			if deathRead(t, filepath.Join(facts.TargetPath, "skills", deathComponent, "SKILL.md")) != deathSkill(deathComponent, deathNewVersion) {
				return nil // The old Prepared host handoff precedes this replacement.
			}
			deathEmitAndBlock(t, deathProveBoundary(t, base, boundary, before, oldConfig))
			return fmt.Errorf("unreachable parent-controlled boundary")
		}
	} else {
		adapter = deathBoundaryCodex{Adapter: codex.New(), beforeMCPLock: func(paths nativeconfig.Paths, codec nativeconfig.Codec) (func() error, error) {
			if codec != nativeconfig.CodecCodex || paths.JSON != filepath.Join(project, ".codex", "config.toml") || paths.JSONC != "" {
				t.Fatalf("primitive boundary selected another codec/path: %+v, %s", paths, codec)
			}
			deathEmitAndBlock(t, deathProveBoundary(t, base, boundary, before, oldConfig))
			return nil, fmt.Errorf("unreachable parent-controlled boundary")
		}}
	}
	engine, _ := deathEngine(t, base, project, adapter, committed)
	deathApply(t, engine, deathRequest(t, engine, base, project, OpUpdate))
	t.Fatal("child SDK update completed without reaching the selected kill boundary")
}

func deathKillAtBarrier(t *testing.T, base, boundary string, verify func(deathBarrier)) deathBarrier {
	t.Helper()
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, executable, "-test.run=^TestProjectProcessDeathChild$", "-test.timeout=25s", "--", base, boundary)
	stdin, err := command.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	stdout, err := command.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	var stderr deathChildOutput
	command.Stderr = &stderr
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	waited := false
	defer func() {
		if !waited {
			_ = command.Process.Kill()
			_ = stdin.Close()
			_ = command.Wait()
		} else {
			_ = stdin.Close()
		}
	}()
	type response struct {
		barrier deathBarrier
		err     error
	}
	ready := make(chan response, 1)
	go func() {
		line, err := bufio.NewReader(io.LimitReader(stdout, 4096)).ReadBytes('\n')
		var barrier deathBarrier
		if err == nil {
			err = json.Unmarshal(line, &barrier)
		}
		ready <- response{barrier, err}
	}()
	select {
	case observed := <-ready:
		if observed.err != nil {
			t.Fatalf("child did not reach %s: %v; stderr=%s", boundary, observed.err, stderr.String())
		}
		verify(observed.barrier)
		if err := command.Process.Kill(); err != nil {
			t.Fatal(err)
		}
		err := command.Wait()
		waited = true
		var exit *exec.ExitError
		if !errors.As(err, &exit) || command.ProcessState.Success() {
			t.Fatalf("child was not killed ungracefully: %v; stderr=%s", err, stderr.String())
		}
		return observed.barrier
	case <-ctx.Done():
		t.Fatalf("child kill boundary deadline: %v; stderr=%s", ctx.Err(), stderr.String())
	}
	return deathBarrier{}
}

// A failed child must not race diagnostic reads or grow parent memory unbounded.
type deathChildOutput struct {
	mu   sync.Mutex
	body bytes.Buffer
}

func (out *deathChildOutput) Write(body []byte) (int, error) {
	out.mu.Lock()
	defer out.mu.Unlock()
	size := len(body)
	remaining := 64*1024 - out.body.Len()
	if remaining > 0 {
		if len(body) > remaining {
			body = body[:remaining]
		}
		_, _ = out.body.Write(body)
	}
	return size, nil
}
func (out *deathChildOutput) String() string {
	out.mu.Lock()
	defer out.mu.Unlock()
	return out.body.String()
}
