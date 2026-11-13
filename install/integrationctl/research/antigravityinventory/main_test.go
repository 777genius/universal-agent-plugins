package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

func fixture(t *testing.T) request {
	t.Helper()
	root, err := os.MkdirTemp(t.TempDir(), "uap-agy-direct-")
	if err != nil {
		t.Fatal(err)
	}
	binary := filepath.Join(t.TempDir(), "agy-inert")
	body := []byte("inert; never executed")
	if err = os.WriteFile(binary, body, 0700); err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(body)
	nativeSHA = hex.EncodeToString(digest[:])
	sourceCommit, sourceTree = strings.Repeat("a", 40), strings.Repeat("b", 40)
	return request{Binary: binary, BinarySHA: hex.EncodeToString(digest[:]), Root: root, Selector: "owned-physical", Action: "list", Commit: sourceCommit, Tree: sourceTree, Deadline: time.Now().Add(29 * time.Second).UnixNano()}
}
func TestClosedRequestRefusesUnknownDuplicateAndWrongTypes(t *testing.T) {
	req := fixture(t)
	body, _ := json.Marshal(req)
	if _, err := decode(body); err != nil {
		t.Fatal(err)
	}
	for _, bad := range []string{string(body) + "{}", strings.Replace(string(body), `"action":"list"`, `"action":"list","action":"enable"`, 1), strings.Replace(string(body), `"action":"list"`, `"action":true`, 1), strings.Replace(string(body), `"action":"list"`, `"action":"install"`, 1), strings.Replace(string(body), `"action":"list"`, `"unexpected":true`, 1), strings.Replace(string(body), `"selector":"owned-physical"`, `"selector":"../foreign"`, 1)} {
		if _, err := decode([]byte(bad)); err == nil {
			t.Fatal("malformed request admitted", bad)
		}
	}
}
func TestCommandOnlyAdmitsSettledBoundedPinnedResult(t *testing.T) {
	for _, kind := range []string{"pass", "cleanup", "no-child", "stderr", "ceiling", "error", "cancel", "binary", "root"} {
		t.Run(kind, func(t *testing.T) {
			req := fixture(t)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			rootRenameDenied := false
			result, err := execute(ctx, req, func(_ context.Context, cmd ports.Command, grace time.Duration) (ports.CommandResult, error) {
				if grace != 5*time.Second || cmd.StdoutLimitBytes != 1<<20 || strings.Join(cmd.Argv[1:], " ") != "plugin list --json" || cmd.Dir != filepath.Join(req.Root, "project") {
					t.Fatal("fixed command differs", cmd)
				}
				for _, env := range cmd.Env {
					if strings.HasPrefix(env, "AGENTPLUGINS_") || strings.HasPrefix(env, "OPENAI_") {
						t.Fatal("ambient credential environment")
					}
				}
				value := ports.CommandResult{CleanupComplete: true, Stdout: []byte("[]\n")}
				switch kind {
				case "cleanup":
					value.CleanupComplete = false
				case "no-child":
					value.NoChildStarted = true
				case "stderr":
					value.Stderr = []byte("private")
				case "ceiling":
					value.Stdout = make([]byte, 1<<20)
				case "error":
					return value, errors.New("private native error")
				case "cancel":
					cancel()
				case "binary":
					if e := os.WriteFile(req.Binary, []byte("changed executable"), 0700); e != nil {
						t.Fatal(e)
					}
				case "root":
					if e := os.Rename(req.Root, req.Root+"-old"); e != nil {
						// Windows keeps the directory handle opened by execute locked.
						// A denied rename means the original root is still held; do
						// not mistake this platform protection for an E2E failure.
						if runtime.GOOS != "windows" || !errors.Is(e, syscall.Errno(32)) {
							t.Fatal(e)
						}
						rootRenameDenied = true
						break
					}
					if e := os.Mkdir(req.Root, 0700); e != nil {
						t.Fatal(e)
					}
				}
				return value, nil
			})
			if kind == "pass" || (kind == "root" && rootRenameDenied) {
				if err != nil || len(result) == 0 {
					t.Fatal(err)
				}
				var value map[string]any
				if json.Unmarshal(result, &value) != nil || value["cleanup_complete"] != true || value["stdout_bytes"] != float64(3) {
					t.Fatal("closed receipt differs")
				}
			} else if err == nil || result != nil {
				t.Fatal("unproved result admitted", kind)
			}
			if rootRenameDenied {
				if _, e := os.Stat(req.Root); e != nil {
					t.Fatal("locked root disappeared", e)
				}
				if _, e := os.Stat(req.Root + "-old"); !errors.Is(e, os.ErrNotExist) {
					t.Fatal("root moved despite rejected rename", e)
				}
			}
		})
	}
}

func TestParentEOFOrPrefetchedExtraCancelsWithoutReceipt(t *testing.T) {
	for _, extra := range []bool{false, true} {
		req := fixture(t)
		input, control := io.Pipe()
		var output bytes.Buffer
		body, _ := json.Marshal(req)
		done := make(chan error, 1)
		started := make(chan struct{})
		go func() {
			done <- run(input, &output, func(ctx context.Context, _ ports.Command, _ time.Duration) (ports.CommandResult, error) {
				close(started)
				<-ctx.Done()
				return ports.CommandResult{CleanupComplete: true}, ctx.Err()
			})
		}()
		if _, err := control.Write(append(body, '\n')); err != nil {
			t.Fatal(err)
		}
		<-started
		if extra {
			if _, err := control.Write([]byte("unexpected")); err != nil {
				t.Fatal(err)
			}
		} else {
			if err := control.Close(); err != nil {
				t.Fatal(err)
			}
		}
		select {
		case err := <-done:
			if err == nil || output.Len() != 0 {
				t.Fatal("parent loss admitted receipt", err)
			}
		case <-time.After(time.Second):
			t.Fatal("parent loss did not cancel")
		}
		if err := control.Close(); err != nil {
			t.Fatal(err)
		}
		if err := input.Close(); err != nil {
			t.Fatal(err)
		}
	}
}

func TestVersionUsesOnlyPinnedFixedAction(t *testing.T) {
	req := fixture(t)
	req.Action = "version"
	body, err := json.Marshal(req)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = decode(body); err != nil {
		t.Fatal(err)
	}
	result, err := execute(context.Background(), req, func(_ context.Context, cmd ports.Command, grace time.Duration) (ports.CommandResult, error) {
		if len(cmd.Argv) != 2 || cmd.Argv[0] != req.Binary || cmd.Argv[1] != "--version" || grace != 5*time.Second {
			t.Fatal("version authority differs", cmd)
		}
		return ports.CommandResult{CleanupComplete: true, Stdout: []byte("1.5.3\n")}, nil
	})
	if err != nil || len(result) == 0 {
		t.Fatal("version refused", err)
	}
}

func publicFixture(t *testing.T) request {
	t.Helper()
	req := fixture(t)
	name := "agy"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	if err := os.Mkdir(filepath.Join(req.Root, "bin"), 0700); err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(req.Binary)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(req.Root, "bin", name), body, 0700); err != nil {
		t.Fatal(err)
	}
	req.Binary = filepath.Join(t.TempDir(), "inert-uap")
	body = []byte("inert compiled public UAP fixture; never executed")
	if err = os.WriteFile(req.Binary, body, 0700); err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(body)
	installerSHA = hex.EncodeToString(digest[:])
	req.BinarySHA, req.Action, req.Selector = installerSHA, "uap-repeat", "uap-agy-native-smoke"
	req.Deadline = time.Now().Add(234 * time.Second).UnixNano()
	return req
}

func TestPublicActionsHaveFixedSelectorsArgvPinsAndEnvironment(t *testing.T) {
	req := publicFixture(t)
	t.Setenv("OPENAI_API_KEY", "inert-not-forwarded")
	source := "https://github.com/777genius/universal-agent-plugins/tree/" + sourceCommit + "/repotests/testdata/agentplugins_native_probe"
	local := filepath.Join(req.Root, "source")
	common := []string{"--target", "antigravity", "--scope", "user", "--yes", "--format", "json"}
	for action, args := range map[string][]string{
		"uap-add": {"add", local, "--no-save"}, "uap-repeat": {"add", local, "--no-save"},
		"uap-missing-backend": {"add", local, "--no-save"},
		"uap-update":          {"update", "uap-agy-native-smoke"}, "uap-repair": {"repair", "uap-agy-native-smoke"},
		"uap-remove": {"remove", "uap-agy-native-smoke"}, "uap-saved-add": {"add", source},
		"uap-cold-install": {"install", "--locked"}, "uap-cold-repeat": {"install", "--locked"},
	} {
		req.Action, req.Selector = action, publicSelector(action)
		if err := validateRequest(req); err != nil {
			t.Fatal(action, err)
		}
		tail := common
		if action == "uap-cold-install" || action == "uap-cold-repeat" {
			tail = []string{"--scope", "user", "--yes", "--format", "json"}
		}
		want := append(append([]string{req.Binary}, args...), tail...)
		if !slices.Equal(fixedArgv(req), want) {
			t.Fatal("fixed public argv differs", action, fixedArgv(req), want)
		}
		for _, entry := range commandEnvironment(req) {
			if strings.HasPrefix(entry, "OPENAI_") || strings.HasPrefix(entry, "GITHUB_TOKEN=") {
				t.Fatal("credential inherited", entry)
			}
		}
		bad := req
		bad.Selector = "other"
		if validateRequest(bad) == nil {
			t.Fatal("arbitrary public selector admitted")
		}
		bad = req
		bad.BinarySHA = nativeSHA
		if validateRequest(bad) == nil {
			t.Fatal("native pin admitted as UAP")
		}
	}
}

func TestPublicNegativeExitStillRequiresStrictFamilyAndRuntimeProof(t *testing.T) {
	for _, kind := range []string{"negative", "cleanup", "no-child", "error", "context", "stdout-bound", "stderr-bound", "native-pin"} {
		t.Run(kind, func(t *testing.T) {
			req := publicFixture(t)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			body, err := execute(ctx, req, func(_ context.Context, cmd ports.Command, grace time.Duration) (ports.CommandResult, error) {
				if grace != 5*time.Second || cmd.Dir != filepath.Join(req.Root, "project") || commandBudget(req.Action) != 235*time.Second {
					t.Fatal("original public budget or cwd changed", cmd)
				}
				value := ports.CommandResult{CleanupComplete: true, ExitCode: 1, Stdout: []byte("refused"), Stderr: []byte("fixed failure")}
				switch kind {
				case "cleanup":
					value.CleanupComplete = false
				case "no-child":
					value.NoChildStarted = true
				case "error":
					return value, errors.New("capture failure")
				case "context":
					cancel()
				case "stdout-bound":
					value.Stdout = make([]byte, 1<<20)
				case "stderr-bound":
					value.Stderr = make([]byte, 32768)
				case "native-pin":
					name := "agy"
					if runtime.GOOS == "windows" {
						name += ".exe"
					}
					if e := os.WriteFile(filepath.Join(req.Root, "bin", name), []byte("changed"), 0700); e != nil {
						t.Fatal(e)
					}
				}
				return value, nil
			})
			if kind != "negative" {
				if err == nil || body != nil {
					t.Fatal("negative exit became a cleanup proof", err)
				}
				return
			}
			var receipt map[string]any
			if err != nil || json.Unmarshal(body, &receipt) != nil || len(receipt) != 14 || receipt["exit"] != float64(1) || receipt["cleanup_complete"] != true || receipt["stderr_bytes"] != float64(13) {
				t.Fatal("settled negative receipt differs", err, receipt)
			}
		})
	}
}
