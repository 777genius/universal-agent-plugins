package main

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/managedstdio"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/pathcontract"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

func fixedFixture(t *testing.T) request {
	t.Helper()
	root, err := os.MkdirTemp("", "uap-cline-session-")
	if err != nil {
		t.Fatal(err)
	}
	root, err = filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.RemoveAll(root); err != nil {
			t.Error(err)
		}
	})
	previous := []string{sourceCommit, sourceTree, nativeSHA, installerSHA, probeSHA, gitBinary, gitSHA, goBinary, goSHA}
	t.Cleanup(func() {
		sourceCommit, sourceTree, nativeSHA, installerSHA, probeSHA, gitBinary, gitSHA, goBinary, goSHA = previous[0], previous[1], previous[2], previous[3], previous[4], previous[5], previous[6], previous[7], previous[8]
	})
	sourceCommit = strings.Repeat("a", 40)
	sourceTree = strings.Repeat("b", 40)
	sum := sha256.Sum256([]byte("inert"))
	nativeSHA = hex.EncodeToString(sum[:])
	installerSHA = nativeSHA
	probeSHA = nativeSHA
	gitSHA = nativeSHA
	goSHA = nativeSHA
	gitBinary = filepath.Join(root, "bin", "git"+suffix())
	goBinary = filepath.Join(root, "bin", "go"+suffix())
	uap, native, probe := paths(root)
	for _, path := range []string{uap, native, probe, gitBinary, goBinary} {
		if err = os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			t.Fatal(err)
		}
		if err = os.WriteFile(path, []byte("inert"), 0700); err != nil {
			t.Fatal(err)
		}
	}
	return request{Schema: 1, Root: root, Action: "native-version", Commit: sourceCommit, Tree: sourceTree, Deadline: time.Now().Add(time.Minute).UnixNano()}
}
func requestPipe(t *testing.T, r request) (*io.PipeReader, *io.PipeWriter) {
	t.Helper()
	reader, writer := io.Pipe()
	body, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	go func() { _, _ = writer.Write(append(body, '\n')) }()
	t.Cleanup(func() { _ = reader.Close(); _ = writer.Close() })
	return reader, writer
}
func TestFixedWireRejectsUnknownDuplicateAndUnpinnedSource(t *testing.T) {
	r := fixedFixture(t)
	body, _ := json.Marshal(r)
	for _, raw := range []string{strings.Replace(string(body), `"action":`, `"extra":0,"action":`, 1), strings.Replace(string(body), `"action":`, `"action":"native-version","action":`, 1), strings.Replace(string(body), r.Commit, strings.Repeat("c", 40), 1), string(body) + "{}"} {
		if _, err := decode([]byte(raw)); err == nil {
			t.Fatal("admitted malformed request")
		}
	}
	r.Action = "arbitrary-command"
	if _, err := command(r); err == nil {
		t.Fatal("unknown action admitted")
	}
}
func TestRunAdmitsOnlySettledPinnedBoundedResult(t *testing.T) {
	for _, condition := range []string{"success", "cleanup", "not-started", "error", "pin", "output", "stderr"} {
		t.Run(condition, func(t *testing.T) {
			r := fixedFixture(t)
			input, _ := requestPipe(t, r)
			var output bytes.Buffer
			calls := 0
			err := run(input, &output, func(_ context.Context, cmd ports.Command, _ time.Duration) (ports.CommandResult, error) {
				calls++
				if cmd.StdoutLimitBytes != 1<<20 || cmd.Dir != filepath.Join(r.Root, "a") {
					t.Fatal("command bounds differ")
				}
				if condition == "pin" {
					uap, _, _ := paths(r.Root)
					if e := os.WriteFile(uap, []byte("changed"), 0700); e != nil {
						t.Fatal(e)
					}
				}
				result := ports.CommandResult{CleanupComplete: condition != "cleanup", NoChildStarted: condition == "not-started", Stdout: []byte("3.0.73")}
				if condition == "output" {
					result.Stdout = make([]byte, (1<<20)+1)
				}
				if condition == "stderr" {
					result.Stderr = make([]byte, 32769)
				}
				if condition == "error" {
					return result, io.ErrUnexpectedEOF
				}
				return result, nil
			})
			if condition == "success" {
				if err != nil || output.Len() == 0 {
					t.Fatalf("valid result: %v", err)
				}
			} else if err == nil || output.Len() != 0 {
				t.Fatal("unproved result published")
			}
			if calls != 1 {
				t.Fatal("unexpected retry")
			}
		})
	}
}
func TestColdEnvironmentHasSeparateRootsAndNoDuplicateKeys(t *testing.T) {
	r := fixedFixture(t)
	r.Action = "cold-session"
	cmd, err := command(r)
	if err != nil || cmd.Dir != filepath.Join(r.Root, "cold") {
		t.Fatal("cold working directory differs", err)
	}
	env := map[string]string{}
	for _, item := range cmd.Env {
		key, value, _ := strings.Cut(item, "=")
		key = strings.ToUpper(key)
		if _, exists := env[key]; exists {
			t.Fatal("duplicate native environment key", key)
		}
		env[key] = value
	}
	for key, leaf := range map[string]string{"HOME": "cold-home", "USERPROFILE": "cold-home", "AGENTPLUGINS_HOME": "cold-cache", "GOCACHE": "cold-go-cache", "GOPATH": "cold-go-path"} {
		if env[key] != filepath.Join(r.Root, leaf) {
			t.Fatal("cold fixture reused warm state", key)
		}
	}
	merged, err := protocolEnvironment(r.Root, "cold-session", transport{Env: map[string]string{"GOWORK": "off", "GOTOOLCHAIN": "local"}})
	if err != nil || strings.Count(strings.Join(merged, "\n"), "GOWORK=") != 1 {
		t.Fatal("protocol environment appended aliases", err)
	}
}

func TestProtocolAgainstRealOwnedProbePreservesMarker(t *testing.T) {
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	probe := filepath.Join(root, "probe"+suffix())
	source, err := filepath.Abs("../../../../../repotests/testdata/agentplugins_native_probe/main.go")
	if err != nil {
		t.Fatal(err)
	}
	buildContext, buildCancel := context.WithTimeout(context.Background(), time.Minute)
	defer buildCancel()
	build := exec.CommandContext(buildContext, "go", "build", "-o", probe, source)
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build exact owned fixture: %v: %s", err, output)
	}
	data, events := filepath.Join(root, "data"), filepath.Join(root, "events.jsonl")
	reported := make([]map[string]any, 0, 2)
	for _, revision := range []string{"0", "1"} {
		transport := transport{CWD: root, Env: map[string]string{"PLUGIN_ROOT": root, "PLUGIN_DATA": data, "UAP_TEST_ROOT": root,
			"UAP_TEST_EVENTS": events, "UAP_TEST_NONCE": "owned", "UAP_TEST_REVISION": revision, "UAP_TEST_REPORT_OPERATION": "1"}}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		cmd := exec.CommandContext(ctx, probe)
		cmd.Dir = root
		cmd.Env, err = protocolEnvironment(root, "direct-session", transport)
		if err != nil {
			t.Fatal(err)
		}
		input, err := cmd.StdinPipe()
		if err != nil {
			t.Fatal(err)
		}
		output, err := cmd.StdoutPipe()
		if err != nil {
			t.Fatal(err)
		}
		if err = cmd.Start(); err != nil {
			t.Fatal(err)
		}
		client := rpcClient{input: input, output: bufio.NewReaderSize(output, 65537)}
		operation := "read"
		if revision == "0" {
			operation = "write"
		}
		exchangeErr := exchange(&client, transport, operation)
		_ = input.Close()
		tail, readErr := io.ReadAll(output)
		waitErr := cmd.Wait()
		cancel()
		if exchangeErr != nil || readErr != nil || waitErr != nil || len(tail) != 0 || len(client.transcript) != 4 {
			t.Fatalf("real fixture exchange %s: %v %v %v", revision, exchangeErr, readErr, waitErr)
		}
		var report bytes.Buffer
		if err := json.NewEncoder(&report).Encode(map[string]any{"facts": client.facts}); err != nil {
			t.Fatal(err)
		}
		var published struct {
			Facts map[string]any `json:"facts"`
		}
		if err := json.Unmarshal(report.Bytes(), &published); err != nil {
			t.Fatal(err)
		}
		reported = append(reported, published.Facts)
	}
	body, err := os.ReadFile(filepath.Join(data, "native-marker.txt"))
	if err != nil || string(body) != "kept-owned" {
		t.Fatal("marker was not preserved", err)
	}
	body, err = os.ReadFile(events)
	if err != nil || len(reported) != 2 || bytes.Count(body, []byte{'\n'}) != 6 {
		t.Fatal("missing original protocol events or reports", err)
	}
	lines := bytes.Split(bytes.TrimSpace(body), []byte{'\n'})
	for index, line := range lines {
		var event struct {
			Method string         `json:"method"`
			Facts  map[string]any `json:"facts"`
		}
		if err := json.Unmarshal(line, &event); err != nil {
			t.Fatal(err)
		}
		if event.Method != []string{"initialize", "tools/list", "tools/call"}[index%3] {
			t.Fatal("original event sequence differs")
		}
		if index%3 != 2 {
			if event.Facts != nil {
				t.Fatal("facts appeared before tools/call")
			}
			continue
		}
		argv, ok := event.Facts["argv"].([]any)
		if !ok || len(argv) != 0 {
			t.Fatal("original no-argument probe did not report an empty array")
		}
		facts := reported[index/3]
		if facts == nil || !reflect.DeepEqual(facts, event.Facts) {
			t.Fatalf("serialized facts differ from original probe event %d: report=%#v event=%#v", index, facts, event.Facts)
		}
	}
}
func TestColdInstallUsesOnlyAgreementTargets(t *testing.T) {
	r := fixedFixture(t)
	uap, _, _ := paths(r.Root)
	project := filepath.Join(r.Root, "cold")
	for _, action := range []string{"cold-locked-install", "cold-repeat", "cold-inspect"} {
		t.Run(action, func(t *testing.T) {
			r.Action = action
			input, _ := requestPipe(t, r)
			want := []string{uap, "install", "--locked", "--yes", "--format", "json", "--scope", "project", "--project-root", project}
			if action == "cold-inspect" {
				want = []string{uap, "run", "--inspect", "--confirm-client-start", "--target", "cline", "--scope", "project", "--project-root", project}
			}
			var output bytes.Buffer
			calls := 0
			err := run(input, &output, func(_ context.Context, cmd ports.Command, _ time.Duration) (ports.CommandResult, error) {
				calls++
				if !slices.Equal(cmd.Argv, want) || cmd.Dir != project {
					t.Errorf("cold invocation: argv=%q cwd=%q; want argv=%q cwd=%q", cmd.Argv, cmd.Dir, want, project)
				}
				for _, value := range []string{"HOME=" + filepath.Join(r.Root, "cold-home"), "AGENTPLUGINS_HOME=" + filepath.Join(r.Root, "cold-cache")} {
					if !slices.Contains(cmd.Env, value) {
						t.Errorf("cold environment missing %q", value)
					}
				}
				return ports.CommandResult{CleanupComplete: true}, nil
			})
			if err != nil || calls != 1 || output.Len() == 0 {
				t.Fatalf("cold invocation did not settle once: calls=%d err=%v", calls, err)
			}
		})
	}
}
func TestParentEOFOrExtraCancelsActualCommandContext(t *testing.T) {
	for _, extra := range []bool{false, true} {
		t.Run(map[bool]string{false: "eof", true: "extra"}[extra], func(t *testing.T) {
			r := fixedFixture(t)
			input, writer := requestPipe(t, r)
			entered := make(chan struct{})
			done := make(chan error, 1)
			var output bytes.Buffer
			go func() {
				done <- run(input, &output, func(ctx context.Context, _ ports.Command, _ time.Duration) (ports.CommandResult, error) {
					close(entered)
					<-ctx.Done()
					return ports.CommandResult{CleanupComplete: true}, ctx.Err()
				})
			}()
			<-entered
			if extra {
				if _, err := writer.Write([]byte("x")); err != nil {
					t.Fatal(err)
				}
			} else {
				_ = writer.Close()
			}
			select {
			case err := <-done:
				if err == nil || output.Len() != 0 {
					t.Fatal("parent loss published success")
				}
			case <-time.After(time.Second):
				t.Fatal("parent control did not cancel")
			}
		})
	}
}

type inputBuffer struct{ bytes.Buffer }

func (*inputBuffer) Close() error { return nil }
func TestProtocolWaitsForValidatedInitialize(t *testing.T) {
	for _, initial := range []string{`{"jsonrpc":"2.0","id":1,"result":{"protocolVersion":"wrong"}}`, `{"jsonrpc":"2.0","id":2,"result":{}}`, `{"jsonrpc":"2.0","id":1,"id":1,"result":{}}`} {
		var input inputBuffer
		client := rpcClient{input: &input, output: bufio.NewReaderSize(strings.NewReader(initial+"\n"), 65537)}
		if err := exchange(&client, transport{}, "read"); err == nil {
			t.Fatal("bad initialize admitted")
		}
		if strings.Count(input.String(), "\n") != 1 || !strings.Contains(input.String(), `"method":"initialize"`) {
			t.Fatal("sent a request before validating initialize")
		}
	}
}

func TestProtocolStderrBoundThroughActualExecCopy(t *testing.T) {
	if os.Getenv("UAP_TEST_OVERSIZED_STDERR") == "1" {
		if _, err := os.Stderr.Write(bytes.Repeat([]byte("x"), 65536)); err != nil {
			os.Exit(2)
		}
		os.Exit(0)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestProtocolStderrBoundThroughActualExecCopy$")
	cmd.Env = append(os.Environ(), "UAP_TEST_OVERSIZED_STDERR=1")
	var capture boundedBuffer
	cmd.Stderr = &capture
	if err := cmd.Run(); err == nil || capture.Len() > 32768 {
		t.Fatalf("oversized subprocess stderr admitted: error=%v bytes=%d", err, capture.Len())
	}
}

func TestSelectedHelperEscapeAndUnknownEnvironmentRefuse(t *testing.T) {
	r := fixedFixture(t)
	project := filepath.Join(r.Root, "a")
	active := filepath.Join(project, ".uap", "managed", "clients", "cline", "fixture")
	data := filepath.Join(project, ".uap", "plugin-data", "fixture")
	helper := filepath.Join(active, managedstdio.RelativeDirectory, managedstdio.ExecutableName)
	for _, path := range []string{helper, filepath.Join(active, "bin", "probe"+suffix())} {
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("inert"), 0700); err != nil {
			t.Fatal(err)
		}
	}
	var pristine transport
	for _, change := range []string{"none", "escape", "environment"} {
		selected := transport{Type: "stdio", Command: helper, CWD: active,
			Args: managedstdio.Arguments(active, data, active, pathcontract.Plugin, "./bin/probe"+suffix(), nil),
			Env:  map[string]string{"PLUGIN_ROOT": active, "PLUGIN_DATA": data, "UAP_TEST_ROOT": r.Root, "UAP_TEST_NONCE": "nonce", "UAP_TEST_REVISION": "0", "UAP_TEST_EVENTS": filepath.Join(r.Root, "mcp-events.jsonl")}}
		if change == "escape" {
			selected.Command, _, _ = paths(r.Root)
		}
		if change == "environment" {
			selected.Env["TOKEN"] = "unexpected"
		}
		if change == "none" {
			pristine = selected
		}
		_, err := validateSelected(r.Root, project, "direct-session", selected)
		if (err == nil) != (change == "none") {
			t.Fatalf("%s: %v", change, err)
		}
	}
	path := filepath.Join(project, ".uap", "clients", "cline", "mcp.json")
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{"disabled", "autoApprove"} {
		row := map[string]any{"transport": pristine, "disabled": false, "autoApprove": []any{}}
		row[field] = nil
		body, err := json.Marshal(map[string]any{"mcpServers": map[string]any{serverName: row}})
		if err != nil {
			t.Fatal(err)
		}
		if err = os.WriteFile(path, body, 0600); err != nil {
			t.Fatal(err)
		}
		if _, err = selected(r.Root, "direct-session"); err == nil {
			t.Fatalf("explicit null %s admitted", field)
		}
	}
}

func TestProtocolReadDoesNotRewriteRetainedMarker(t *testing.T) {
	target := transport{CWD: "/owned", Env: map[string]string{"UAP_TEST_NONCE": "nonce", "UAP_TEST_REVISION": "1", "PLUGIN_ROOT": "/owned", "PLUGIN_DATA": "/data"}}
	facts := map[string]any{"nonce": "nonce", "revision": "1", "plugin_root": "/owned", "plugin_data": "/data", "cwd": "/owned", "marker": "kept-nonce", "operation": "read"}
	encoded, _ := json.Marshal(facts)
	responses := []map[string]any{
		{"jsonrpc": "2.0", "id": 1, "result": map[string]any{"protocolVersion": "2025-06-18", "serverInfo": map[string]any{"name": "agentplugins-native-probe", "version": "1.0.0"}}},
		{"jsonrpc": "2.0", "id": 2, "result": map[string]any{"tools": []any{map[string]any{"name": "inspect_runtime"}}}},
		{"jsonrpc": "2.0", "id": 3, "result": map[string]any{"isError": false, "content": []any{map[string]any{"type": "text", "text": string(encoded)}}}},
		{"jsonrpc": "2.0", "id": 4, "error": map[string]any{"code": -32602}},
	}
	var output bytes.Buffer
	for _, response := range responses {
		if err := json.NewEncoder(&output).Encode(response); err != nil {
			t.Fatal(err)
		}
	}
	var input inputBuffer
	client := rpcClient{input: &input, output: bufio.NewReaderSize(&output, 65537)}
	if err := exchange(&client, target, "read"); err != nil {
		t.Fatal(err)
	}
	var call struct {
		Params struct {
			Arguments struct {
				Operation string `json:"operation"`
			} `json:"arguments"`
		} `json:"params"`
	}
	if err := json.Unmarshal(bytes.Split(input.Bytes(), []byte("\n"))[3], &call); err != nil {
		t.Fatal(err)
	}
	if call.Params.Arguments.Operation != "read" {
		t.Fatal("retained marker was overwritten")
	}
}
