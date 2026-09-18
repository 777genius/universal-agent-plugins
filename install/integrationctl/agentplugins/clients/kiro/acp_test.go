package kiro

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	legacyports "github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

type fixtureDuplexRunner struct {
	recordingRunner
	output          string
	reader          io.Reader
	duplexErr       error
	postError       error
	command         legacyports.Command
	written         []byte
	exchangeErr     error
	hasDeadline     bool
	keepAlive       bool
	closeAfterWrite bool
	closeOnContext  bool
	writeDelay      time.Duration
	stdinCloses     int
}

type fixtureACPStdin struct {
	bytes.Buffer
	close func() error
}

func (stdin *fixtureACPStdin) Close() error {
	if stdin.close == nil {
		return nil
	}
	return stdin.close()
}

func (runner *fixtureDuplexRunner) RunDuplexWithPlannedShutdown(ctx context.Context, command legacyports.Command, exchange func(io.Writer, io.Reader) error) error {
	runner.command = command
	_, runner.hasDeadline = ctx.Deadline()
	if runner.duplexErr != nil {
		return runner.duplexErr
	}
	reader := runner.reader
	var liveWriter *os.File
	var outputWritten chan struct{}
	var closeOutput func()
	if reader == nil && runner.keepAlive {
		liveReader, writer, err := os.Pipe()
		if err != nil {
			return err
		}
		defer liveReader.Close()
		liveWriter = writer
		reader = liveReader
		outputWritten = make(chan struct{})
		var closeOnce sync.Once
		closeOutput = func() { closeOnce.Do(func() { _ = writer.Close() }) }
		go func() {
			if runner.writeDelay > 0 {
				time.Sleep(runner.writeDelay)
			}
			_, _ = io.WriteString(writer, runner.output)
			close(outputWritten)
			if runner.closeAfterWrite {
				closeOutput()
			}
		}()
		if runner.closeOnContext {
			go func() {
				<-ctx.Done()
				closeOutput()
			}()
		}
	} else if reader == nil {
		reader = strings.NewReader(runner.output)
	}
	stdin := &fixtureACPStdin{close: func() error {
		runner.stdinCloses++
		return nil
	}}
	runner.exchangeErr = exchange(stdin, reader)
	_ = stdin.Close()
	if liveWriter != nil {
		closeOutput()
	}
	runner.written = append([]byte(nil), stdin.Bytes()...)
	if runner.postError != nil {
		return errors.Join(runner.exchangeErr, runner.postError)
	}
	return runner.exchangeErr
}

type terminalErrorReader struct {
	err error
}

type delayedReader struct {
	delay  time.Duration
	reader io.Reader
	once   bool
}

func (reader *delayedReader) Read(value []byte) (int, error) {
	if !reader.once {
		reader.once = true
		time.Sleep(reader.delay)
	}
	return reader.reader.Read(value)
}

type contextBlockingReader struct {
	context.Context
	prefix *bytes.Reader
}

func (reader *contextBlockingReader) Read(buffer []byte) (int, error) {
	if reader.prefix.Len() > 0 {
		return reader.prefix.Read(buffer)
	}
	<-reader.Done()
	return 0, reader.Err()
}

func (reader terminalErrorReader) Read([]byte) (int, error) {
	return 0, reader.err
}

func acpResponse(id int, result string) string {
	return fmt.Sprintf("{\"jsonrpc\":\"2.0\",\"id\":%d,\"result\":%s}\n", id, result)
}

func acpStatus(sessionID, name, status string, tools string) string {
	if tools == "" {
		tools = "null"
	}
	return fmt.Sprintf("{\"jsonrpc\":\"2.0\",\"method\":\"_kiro/mcp/status\",\"params\":{\"sessionId\":%q,\"serverName\":%q,\"status\":%q,\"tools\":%s}}\n", sessionID, name, status, tools)
}

func acpArrayStatus(sessionID, servers string) string {
	return fmt.Sprintf("{\"jsonrpc\":\"2.0\",\"method\":\"_kiro/mcp/status\",\"params\":{\"sessionId\":%q,\"servers\":%s}}\n", sessionID, servers)
}

func acpArrayServer(name, status, tools string) string {
	if tools == "" {
		return fmt.Sprintf(`{"name":%q,"authType":"oauth","status":%q}`, name, status)
	}
	return fmt.Sprintf(`{"name":%q,"authType":"oauth","status":%q,"tools":%s}`, name, status, tools)
}

func connectedACP(name string) string {
	return acpResponse(0, `{"protocolVersion":1}`) +
		acpResponse(1, `{"sessionId":"session-1"}`) +
		acpStatus("session-1", name, "connected", `[{"name":"search","disabled":false}]`)
}

func TestKiroACPVerifiesOneAndMultipleNativeServersWithoutPrompt(t *testing.T) {
	t.Parallel()
	for _, servers := range [][]string{{"alpha"}, {"alpha", "beta"}} {
		servers := servers
		t.Run(strings.Join(servers, "+"), func(t *testing.T) {
			t.Parallel()
			output := acpResponse(0, `{"protocolVersion":1}`)
			for _, name := range servers {
				output += acpStatus("session-1", name, "connected", `[{"name":"tool-`+name+`","disabled":false}]`)
			}
			output += acpResponse(1, `{"sessionId":"session-1"}`)
			runner := &fixtureDuplexRunner{output: output, keepAlive: true}
			cwd := t.TempDir()
			if err := VerifyACP(context.Background(), runner, filepath.Join("test", "kiro-cli"), cwd, servers); err != nil {
				t.Fatal(err)
			}
			wantArgv := []string{filepath.Join("test", "kiro-cli"), "acp", "--agent-engine", "v3", "--auth-method", "cli"}
			if !reflect.DeepEqual(runner.command.Argv, wantArgv) || runner.command.Dir != cwd || !filepath.IsAbs(runner.command.Dir) {
				t.Fatalf("command = %+v, want argv=%v cwd=%s", runner.command, wantArgv, cwd)
			}
			if !runner.hasDeadline {
				t.Fatal("ACP process context has no time bound")
			}
			requests := strings.Split(strings.TrimSpace(string(runner.written)), "\n")
			if len(requests) != 2 || strings.Contains(string(runner.written), "session/prompt") {
				t.Fatalf("ACP requests = %q", runner.written)
			}
			var initialize, session map[string]any
			if json.Unmarshal([]byte(requests[0]), &initialize) != nil || json.Unmarshal([]byte(requests[1]), &session) != nil {
				t.Fatalf("requests are not JSON: %q", requests)
			}
			if initialize["method"] != "initialize" || initialize["id"] != float64(0) || session["method"] != "session/new" || session["id"] != float64(1) {
				t.Fatalf("requests = %#v %#v", initialize, session)
			}
			initializeParams := initialize["params"].(map[string]any)
			clientInfo := initializeParams["clientInfo"].(map[string]any)
			if initializeParams["protocolVersion"] != float64(1) || len(initializeParams["clientCapabilities"].(map[string]any)) != 0 ||
				clientInfo["name"] != "agentplugins" || clientInfo["version"] != "1" {
				t.Fatalf("initialize params = %#v", initializeParams)
			}
			params := session["params"].(map[string]any)
			if params["cwd"] != cwd || len(params["mcpServers"].([]any)) != 0 {
				t.Fatalf("session params = %#v", params)
			}
		})
	}
}

func TestKiroACPAllowsConnectingThenConnected(t *testing.T) {
	t.Parallel()
	runner := &fixtureDuplexRunner{output: acpResponse(0, `{"protocolVersion":1}`) +
		acpStatus("s", "alpha", "connecting", "") + acpResponse(1, `{"sessionId":"s"}`) +
		acpStatus("s", "alpha", "connected", `[{"name":"search","disabled":false}]`), keepAlive: true}
	if err := VerifyACP(context.Background(), runner, "kiro-cli", t.TempDir(), []string{"alpha"}); err != nil {
		t.Fatal(err)
	}
}

func TestKiroACPActualServersArrayOrderingAndDuplicateConnecting(t *testing.T) {
	t.Parallel()
	connecting := acpArrayStatus("s", "["+acpArrayServer("alpha", "connecting", "")+"]")
	connected := acpArrayStatus("s", "["+acpArrayServer("alpha", "connected", `[{"name":"search","disabled":false}]`)+"]")
	runner := &fixtureDuplexRunner{output: acpResponse(0, `{"protocolVersion":1}`) + connecting + connecting +
		acpResponse(1, `{"sessionId":"s"}`) + connected, keepAlive: true}
	if err := VerifyACP(context.Background(), runner, "kiro-cli", t.TempDir(), []string{"alpha"}); err != nil {
		t.Fatalf("captured Kiro ordering failed: %v", err)
	}
}

func TestKiroACPCapturedDuplicateConnectingSnapshotsThenConnectedSnapshot(t *testing.T) {
	t.Parallel()
	connecting := acpArrayStatus("s", "["+acpArrayServer("alpha", "connecting", "")+","+acpArrayServer("beta", "connecting", "")+"]")
	connected := acpArrayStatus("s", "["+
		acpArrayServer("alpha", "connected", `[{"name":"search","disabled":false}]`)+","+
		acpArrayServer("beta", "connected", `[{"name":"fetch","disabled":false}]`)+"]")
	runner := &fixtureDuplexRunner{
		output:    acpResponse(0, `{"protocolVersion":1}`) + connecting + connecting + acpResponse(1, `{"sessionId":"s"}`) + connected,
		keepAlive: true,
	}
	if err := VerifyACP(context.Background(), runner, "kiro-cli", t.TempDir(), []string{"alpha", "beta"}); err != nil {
		t.Fatalf("captured Kiro 2.19.1/KAS 0.48.0 sequence failed: %v", err)
	}
}

func TestKiroACPServersArraySupportsPlannedAndUnrelatedServers(t *testing.T) {
	t.Parallel()
	servers := "[" + acpArrayServer("unrelated", "connecting", "") + "," +
		acpArrayServer("alpha", "connected", `[{"name":"a","disabled":false}]`) + "," +
		acpArrayServer("beta", "connected", `[{"name":"b","disabled":false}]`) + "]"
	runner := &fixtureDuplexRunner{output: acpResponse(0, `{"protocolVersion":1}`) +
		acpArrayStatus("s", servers) + acpResponse(1, `{"sessionId":"s"}`), keepAlive: true}
	if err := VerifyACP(context.Background(), runner, "kiro-cli", t.TempDir(), []string{"alpha", "beta"}); err != nil {
		t.Fatal(err)
	}
}

func TestKiroACPServersArrayRepeatedConnectedSnapshotDuringPeerTransition(t *testing.T) {
	t.Parallel()
	alphaConnecting := acpArrayServer("alpha", "connecting", "")
	betaConnecting := acpArrayServer("beta", "connecting", "")
	alphaConnected := acpArrayServer("alpha", "connected", `[{"name":"a","disabled":false}]`)
	betaConnected := acpArrayServer("beta", "connected", `[{"name":"b","disabled":false}]`)
	runner := &fixtureDuplexRunner{output: acpResponse(0, `{"protocolVersion":1}`) +
		acpArrayStatus("s", "["+alphaConnecting+","+betaConnecting+"]") +
		acpResponse(1, `{"sessionId":"s"}`) +
		acpArrayStatus("s", "["+alphaConnected+","+betaConnecting+"]") +
		acpArrayStatus("s", "["+alphaConnected+","+betaConnected+"]"), keepAlive: true}
	if err := VerifyACP(context.Background(), runner, "kiro-cli", t.TempDir(), []string{"alpha", "beta"}); err != nil {
		t.Fatalf("repeated connected entry in a later registry snapshot failed: %v", err)
	}
}

func TestKiroACPFullSnapshotOmissionRevokesConnectedState(t *testing.T) {
	t.Parallel()
	alphaConnected := acpArrayServer("alpha", "connected", `[{"name":"a","disabled":false}]`)
	betaConnected := acpArrayServer("beta", "connected", `[{"name":"b","disabled":false}]`)
	runner := &fixtureDuplexRunner{output: acpResponse(0, `{"protocolVersion":1}`) +
		acpResponse(1, `{"sessionId":"s"}`) +
		acpArrayStatus("s", "["+alphaConnected+","+betaConnected+"]") +
		acpArrayStatus("s", "["+alphaConnected+"]"), keepAlive: true}
	err := VerifyACP(context.Background(), runner, "kiro-cli", t.TempDir(), []string{"alpha", "beta"})
	if !errors.Is(err, shared.ErrRecognizedNegativeEvidence) || !strings.Contains(err.Error(), "omitted planned server beta") {
		t.Fatalf("connected -> omitted full snapshot error = %v", err)
	}
}

func TestKiroACPRejectsMalformedOrAmbiguousServersArrays(t *testing.T) {
	t.Parallel()
	valid := acpArrayServer("alpha", "connected", `[{"name":"search","disabled":false}]`)
	tests := map[string]string{
		"both shapes":                 `{"sessionId":"s","serverName":"alpha","servers":[` + valid + `]}`,
		"duplicate names":             `{"sessionId":"s","servers":[` + valid + `,` + valid + `]}`,
		"malformed array":             `{"sessionId":"s","servers":{}}`,
		"malformed record":            `{"sessionId":"s","servers":["alpha"]}`,
		"missing name":                `{"sessionId":"s","servers":[{"status":"connected","tools":[]}]}`,
		"malformed auth":              `{"sessionId":"s","servers":[{"name":"alpha","authType":7,"status":"connected","tools":[]}]}`,
		"unrelated malformed status":  `{"sessionId":"s","servers":[{"name":"unrelated","status":7},` + valid + `]}`,
		"unrelated malformed tools":   `{"sessionId":"s","servers":[{"name":"unrelated","status":"connected","tools":{}},` + valid + `]}`,
		"conflicting record identity": `{"sessionId":"s","servers":[{"name":"alpha","serverName":"beta","status":"connected","tools":[{"name":"search","disabled":false}]}]}`,
		"conflicting duplicate connecting": `{"sessionId":"s","servers":[` + acpArrayServer("alpha", "connecting", "") + `]}` + "\n" +
			`{"jsonrpc":"2.0","method":"_kiro/mcp/status","params":{"sessionId":"s","servers":[{"name":"alpha","authType":"iam","status":"connecting"}]}}`,
	}
	for name, params := range tests {
		params := params
		t.Run(name, func(t *testing.T) {
			status := `{"jsonrpc":"2.0","method":"_kiro/mcp/status","params":` + params + "}\n"
			if name == "conflicting duplicate connecting" {
				parts := strings.SplitN(params, "\n", 2)
				status = `{"jsonrpc":"2.0","method":"_kiro/mcp/status","params":` + parts[0] + "}\n" + parts[1] + "\n"
			}
			runner := &fixtureDuplexRunner{output: acpResponse(0, `{"protocolVersion":1}`) + status + acpResponse(1, `{"sessionId":"s"}`)}
			if err := VerifyACP(context.Background(), runner, "kiro-cli", t.TempDir(), []string{"alpha"}); err == nil {
				t.Fatal("ambiguous/malformed servers array unexpectedly succeeded")
			}
		})
	}
}

func TestKiroACPStartupTimeoutAllowsDelayedValidColdStart(t *testing.T) {
	runner := &fixtureDuplexRunner{output: connectedACP("alpha"), keepAlive: true, writeDelay: 25 * time.Millisecond}
	if err := verifyACPWithTimeout(context.Background(), runner, "kiro-cli", t.TempDir(), []string{"alpha"}, 250*time.Millisecond); err != nil {
		t.Fatalf("delayed valid ACP startup failed: %v", err)
	}
}

func TestKiroACPRealPipeDeadlineBoundary(t *testing.T) {
	complete := connectedACP("alpha")
	for name, test := range map[string]struct {
		output  string
		wantErr bool
	}{
		"complete record then EOF": {output: complete, wantErr: true},
		"early EOF":                {output: acpResponse(0, `{"protocolVersion":1}`), wantErr: true},
		"partial record then EOF":  {output: complete + `{"jsonrpc":"2.0"`, wantErr: true},
		"complete contradiction":   {output: complete + acpStatus("session-1", "alpha", "disconnected", ""), wantErr: true},
	} {
		t.Run(name, func(t *testing.T) {
			runner := &fixtureDuplexRunner{output: test.output, keepAlive: true, closeAfterWrite: true}
			err := verifyACPWithTimeout(context.Background(), runner, "kiro-cli", t.TempDir(), []string{"alpha"}, 250*time.Millisecond)
			if test.wantErr && !errors.Is(err, shared.ErrRecognizedNegativeEvidence) {
				t.Fatalf("real-pipe error = %v, want negative evidence", err)
			}
			if !test.wantErr && err != nil {
				t.Fatalf("real-pipe completion failed: %v", err)
			}
		})
	}

	t.Run("no data timeout", func(t *testing.T) {
		runner := &fixtureDuplexRunner{keepAlive: true, closeOnContext: true}
		err := verifyACPWithTimeout(context.Background(), runner, "kiro-cli", t.TempDir(), []string{"alpha"}, 25*time.Millisecond)
		if !errors.Is(err, context.DeadlineExceeded) || !errors.Is(err, shared.ErrRecognizedNegativeEvidence) {
			t.Fatalf("real-pipe no-data error = %v, want bounded negative deadline evidence", err)
		}
	})
}

func TestKiroACPRealPipePostSuccessSettlement(t *testing.T) {
	for name, trailing := range map[string]string{
		"complete contradiction": acpArrayStatus("s", "["+acpArrayServer("alpha", "disconnected", "")+"]"),
		"partial record":         `{"jsonrpc":"2.0"`,
	} {
		trailing := trailing
		t.Run(name, func(t *testing.T) {
			stdout, peer, err := os.Pipe()
			if err != nil {
				t.Fatal(err)
			}
			defer stdout.Close()
			defer peer.Close()
			stdinPeer, stdin, err := os.Pipe()
			if err != nil {
				t.Fatal(err)
			}
			defer stdinPeer.Close()
			defer stdin.Close()
			go func() {
				_, _ = io.WriteString(peer, acpResponse(0, `{"protocolVersion":1}`)+acpResponse(1, `{"sessionId":"s"}`)+
					acpArrayStatus("s", "["+acpArrayServer("alpha", "connected", `[{"name":"search","disabled":false}]`)+"]"))
				time.Sleep(kiroACPSettlement / 3)
				_, _ = io.WriteString(peer, trailing)
			}()
			err = exchangeKiroACP(stdin, stdout, t.TempDir(), map[string]*kiroACPServerState{"alpha": {}})
			if !errors.Is(err, shared.ErrRecognizedNegativeEvidence) && !errors.Is(err, ErrACPPartialExit) {
				t.Fatalf("settlement error = %v, want trailing negative evidence", err)
			}
		})
	}

	t.Run("quiet long-lived peer", func(t *testing.T) {
		stdout, peer, err := os.Pipe()
		if err != nil {
			t.Fatal(err)
		}
		defer stdout.Close()
		defer peer.Close()
		stdinPeer, stdin, err := os.Pipe()
		if err != nil {
			t.Fatal(err)
		}
		defer stdinPeer.Close()
		defer stdin.Close()
		go func() {
			_, _ = io.WriteString(peer, acpResponse(0, `{"protocolVersion":1}`)+acpResponse(1, `{"sessionId":"s"}`)+
				acpArrayStatus("s", "["+acpArrayServer("alpha", "connected", `[{"name":"search","disabled":false}]`)+"]"))
		}()
		started := time.Now()
		if err := exchangeKiroACP(stdin, stdout, t.TempDir(), map[string]*kiroACPServerState{"alpha": {}}); err != nil {
			t.Fatal(err)
		}
		if elapsed := time.Since(started); elapsed < kiroACPSettlement/2 || elapsed > time.Second {
			t.Fatalf("settlement duration = %s, want bounded quiet drain", elapsed)
		}
	})
}

func TestKiroACPSettlementBoundaryObservesAlreadyQueuedPipeEvidence(t *testing.T) {
	if runtime.GOOS != "linux" && runtime.GOOS != "darwin" {
		t.Skip("kernel pipe settlement probe is implemented on Linux and Darwin")
	}
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	if _, err := io.WriteString(writer, "queued-before-boundary"); err != nil {
		t.Fatal(err)
	}
	queued, eof, err := queuedACPRealPipeEvidence(reader)
	if err != nil || !queued || eof {
		t.Fatalf("queued boundary evidence = queued:%v eof:%v error:%v", queued, eof, err)
	}
	buffer := make([]byte, len("queued-before-boundary"))
	if _, err := io.ReadFull(reader, buffer); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	queued, eof, err = queuedACPRealPipeEvidence(reader)
	if err != nil || queued || !eof {
		t.Fatalf("EOF boundary evidence = queued:%v eof:%v error:%v", queued, eof, err)
	}
}

func TestKiroACPConsumesQueuedStatusesUntilCleanEOFAfterCompletion(t *testing.T) {
	t.Parallel()
	connected := acpStatus("s", "alpha", "connected", `[{"name":"search","disabled":false}]`)
	for name, queued := range map[string]string{
		"disconnect":            acpStatus("s", "alpha", "disconnected", ""),
		"conflicting duplicate": acpStatus("s", "alpha", "connected", `[{"name":"different","disabled":false}]`),
		"regression":            acpStatus("s", "alpha", "connecting", ""),
	} {
		queued := queued
		t.Run(name, func(t *testing.T) {
			runner := &fixtureDuplexRunner{output: acpResponse(0, `{"protocolVersion":1}`) + acpResponse(1, `{"sessionId":"s"}`) + connected + queued, keepAlive: true}
			err := VerifyACP(context.Background(), runner, "kiro-cli", t.TempDir(), []string{"alpha"})
			if !errors.Is(err, shared.ErrRecognizedNegativeEvidence) {
				t.Fatalf("error = %v, want queued contradictory status to be recognized negative evidence", err)
			}
			if runner.stdinCloses != 1 {
				t.Fatalf("stdin closes = %d, want exactly one", runner.stdinCloses)
			}
			if strings.Contains(string(runner.written), "session/prompt") {
				t.Fatalf("ACP requests unexpectedly prompted: %q", runner.written)
			}
		})
	}
}

func TestKiroACPRejectsQueuedDuplicateConnectingAfterConnected(t *testing.T) {
	t.Parallel()
	connecting := acpStatus("s", "alpha", "connecting", "")
	connected := acpStatus("s", "alpha", "connected", `[{"name":"search","disabled":false}]`)
	runner := &fixtureDuplexRunner{
		output: acpResponse(0, `{"protocolVersion":1}`) + connecting +
			acpResponse(1, `{"sessionId":"s"}`) + connected + connecting,
		keepAlive: true,
	}
	err := VerifyACP(context.Background(), runner, "kiro-cli", t.TempDir(), []string{"alpha"})
	if !errors.Is(err, shared.ErrRecognizedNegativeEvidence) || !strings.Contains(err.Error(), "regressive connecting status after connected") {
		t.Fatalf("queued connected -> connecting error = %v, want recognized regressive evidence", err)
	}
	if runner.stdinCloses != 1 {
		t.Fatalf("stdin closes = %d, want exactly one after failed settlement", runner.stdinCloses)
	}
}

func TestKiroACPCleanEOFAfterCompletionSucceedsAndClosesStdinOnce(t *testing.T) {
	t.Parallel()
	runner := &fixtureDuplexRunner{output: connectedACP("alpha"), keepAlive: true}
	if err := VerifyACP(context.Background(), runner, "kiro-cli", t.TempDir(), []string{"alpha"}); err != nil {
		t.Fatal(err)
	}
	if runner.stdinCloses != 1 {
		t.Fatalf("stdin closes = %d, want exactly one", runner.stdinCloses)
	}
}

func TestKiroACPRequiresClosableStdinBeforeWritingRequests(t *testing.T) {
	t.Parallel()
	var stdin bytes.Buffer
	err := exchangeKiroACP(&stdin, strings.NewReader(connectedACP("alpha")), t.TempDir(), map[string]*kiroACPServerState{"alpha": {}})
	if err == nil || !strings.Contains(err.Error(), "stdin does not support close") {
		t.Fatalf("error = %v, want closable-stdin contract failure", err)
	}
	if stdin.Len() != 0 {
		t.Fatalf("non-closable stdin received %d request bytes", stdin.Len())
	}
}

func TestKiroACPRejectsPartialRecordAfterCompletion(t *testing.T) {
	t.Parallel()
	runner := &fixtureDuplexRunner{output: connectedACP("alpha") + `{"jsonrpc":"2.0"`, keepAlive: true}
	err := VerifyACP(context.Background(), runner, "kiro-cli", t.TempDir(), []string{"alpha"})
	if !errors.Is(err, ErrACPPartialExit) || !errors.Is(err, shared.ErrRecognizedNegativeEvidence) {
		t.Fatalf("error = %v, want partial trailing record failure", err)
	}
	if runner.stdinCloses != 1 {
		t.Fatalf("stdin closes = %d, want exactly one", runner.stdinCloses)
	}
}

func TestKiroACPNonClosingOutputFailsAtContextBound(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	runner := &fixtureDuplexRunner{reader: &contextBlockingReader{Context: ctx, prefix: bytes.NewReader([]byte(connectedACP("alpha")))}}
	started := time.Now()
	err := VerifyACP(ctx, runner, "kiro-cli", t.TempDir(), []string{"alpha"})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("error = %v, want context deadline failure", err)
	}
	if elapsed := time.Since(started); elapsed > time.Second {
		t.Fatalf("non-closing output exceeded bounded failure time: %s", elapsed)
	}
	if runner.stdinCloses != 1 {
		t.Fatalf("stdin closes = %d, want exactly one", runner.stdinCloses)
	}
}

func TestKiroACPProcessFailureAfterCleanCompletionFailsClosed(t *testing.T) {
	t.Parallel()
	processErr := errors.New("ACP process exited unsuccessfully")
	runner := &fixtureDuplexRunner{output: connectedACP("alpha"), postError: processErr}
	err := VerifyACP(context.Background(), runner, "kiro-cli", t.TempDir(), []string{"alpha"})
	if !errors.Is(err, processErr) || !errors.Is(err, shared.ErrRecognizedNegativeEvidence) || errors.Is(err, ErrACPContractUnknown) {
		t.Fatalf("error = %v, want fail-closed post-exchange process failure", err)
	}
}

func TestKiroACPRejectsEveryOutOfRangeNumberToken(t *testing.T) {
	t.Parallel()
	initialize := acpResponse(0, `{"protocolVersion":1}`)
	session := acpResponse(1, `{"sessionId":"s"}`)
	status := acpStatus("s", "alpha", "connected", `[{"name":"search","disabled":false}]`)
	for name, output := range map[string]string{
		"initialize unused member": acpResponse(0, `{"protocolVersion":1,"unused":{"value":1e1000000}}`),
		"session unused member":    initialize + acpResponse(1, `{"sessionId":"s","unused":[1e1000000]}`),
		"notification unused member": initialize + session + strings.Replace(status, `"status":"connected"`,
			`"status":"connected","unused":{"value":1e1000000}`, 1),
	} {
		output := output
		t.Run(name, func(t *testing.T) {
			runner := &fixtureDuplexRunner{output: output, keepAlive: true}
			err := VerifyACP(context.Background(), runner, "kiro-cli", t.TempDir(), []string{"alpha"})
			if !errors.Is(err, shared.ErrRecognizedNegativeEvidence) || errors.Is(err, ErrACPContractUnknown) {
				t.Fatalf("error = %v, want out-of-range JSON number rejected as a protocol failure", err)
			}
		})
	}
}

func TestKiroACPRejectsInvalidUTF8WithoutReplacement(t *testing.T) {
	t.Parallel()
	initialize := []byte(acpResponse(0, `{"protocolVersion":1}`))
	session := []byte(acpResponse(1, `{"sessionId":"s"}`))
	connected := []byte(acpStatus("s", "alpha", "connected", `[{"name":"search","disabled":false}]`))
	invalid := byte(0xff)
	join := func(parts ...[]byte) []byte {
		var result []byte
		for _, part := range parts {
			result = append(result, part...)
		}
		return result
	}
	tests := map[string][]byte{
		"status field": join(initialize, session,
			[]byte(`{"jsonrpc":"2.0","method":"_kiro/mcp/status","params":{"sessionId":"s","serverName":"alpha","status":"`),
			[]byte{invalid}, []byte(`","tools":[{"name":"search","disabled":false}]}}`+"\n")),
		"extension value": join(initialize,
			[]byte(`{"jsonrpc":"2.0","id":1,"result":{"sessionId":"s","extension":"`),
			[]byte{invalid}, []byte(`"}}`+"\n")),
		"object key": join(initialize, session,
			[]byte(`{"jsonrpc":"2.0","method":"_kiro/mcp/status","params":{"sessionId":"s","serverName":"alpha","status":"connected","tools":[{"name":"search","disabled":false}],"`),
			[]byte{invalid}, []byte(`":true}}`+"\n")),
		"partial multibyte boundary": join(initialize, session, connected,
			[]byte{'{', '"', 'x', '"', ':', '"', 0xe2, 0x82}),
		"queued settlement record": join(initialize, session, connected,
			[]byte(`{"jsonrpc":"2.0","method":"progress","params":{"extension":"`),
			[]byte{invalid}, []byte(`"}}`+"\n")),
	}
	for name, output := range tests {
		output := output
		t.Run(name, func(t *testing.T) {
			runner := &fixtureDuplexRunner{output: string(output), keepAlive: true, closeAfterWrite: true}
			err := VerifyACP(context.Background(), runner, "kiro-cli", t.TempDir(), []string{"alpha"})
			if !errors.Is(err, shared.ErrRecognizedNegativeEvidence) {
				t.Fatalf("error = %v, want invalid UTF-8 to prevent connected success", err)
			}
			if err == nil {
				t.Fatal("invalid UTF-8 record reached connected success")
			}
		})
	}
}

func TestKiroACPRejectsUnpairedUTF16EscapesWithoutReplacement(t *testing.T) {
	t.Parallel()
	initialize := acpResponse(0, `{"protocolVersion":1}`)
	session := acpResponse(1, `{"sessionId":"s"}`)
	connected := acpStatus("s", "alpha", "connected", `[{"name":"search","disabled":false}]`)
	tests := map[string]string{
		"tool name":         initialize + session + acpStatus("s", "alpha", "connected", `[{"name":"\ud800","disabled":false}]`),
		"server name":       initialize + session + `{"jsonrpc":"2.0","method":"_kiro/mcp/status","params":{"sessionId":"s","serverName":"\ud800","status":"connected","tools":[{"name":"search","disabled":false}]}}` + "\n",
		"object key":        initialize + session + `{"jsonrpc":"2.0","method":"_kiro/mcp/status","params":{"sessionId":"s","serverName":"alpha","status":"connected","tools":[{"name":"search","disabled":false}],"\ud800":true}}` + "\n",
		"extension value":   initialize + `{"jsonrpc":"2.0","id":1,"result":{"sessionId":"s","extension":"\ud800"}}` + "\n",
		"queued settlement": initialize + session + connected + `{"jsonrpc":"2.0","method":"progress","params":{"extension":"\ud800"}}` + "\n",
		"standalone low":    initialize + `{"jsonrpc":"2.0","id":1,"result":{"sessionId":"\udfff"}}` + "\n",
	}
	for name, output := range tests {
		output := output
		t.Run(name, func(t *testing.T) {
			runner := &fixtureDuplexRunner{output: output, keepAlive: true, closeAfterWrite: true}
			err := VerifyACP(context.Background(), runner, "kiro-cli", t.TempDir(), []string{"alpha"})
			if !errors.Is(err, shared.ErrRecognizedNegativeEvidence) || !strings.Contains(err.Error(), "surrogate") {
				t.Fatalf("error = %v, want unpaired UTF-16 escape rejected before evidence acceptance", err)
			}
		})
	}
}

func TestKiroACPAcceptsValidUTF16SurrogatePairs(t *testing.T) {
	t.Parallel()
	message, err := decodeACPMessage([]byte(`{"jsonrpc":"2.0","method":"progress","params":{"tool":"\ud83d\ude80","\ud83d\ude80":true}}`))
	if err != nil {
		t.Fatalf("valid surrogate pair rejected: %v", err)
	}
	params := message.document["params"].(map[string]any)
	if params["tool"] != "🚀" || params["🚀"] != true {
		t.Fatalf("decoded surrogate-pair strings = %#v", params)
	}
}

func TestKiroACPRealPipeRejectsInvalidUTF8QueuedDuringSettlement(t *testing.T) {
	stdout, peer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer stdout.Close()
	defer peer.Close()
	stdinPeer, stdin, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer stdinPeer.Close()
	defer stdin.Close()
	go func() {
		_, _ = io.WriteString(peer, connectedACP("alpha"))
		time.Sleep(kiroACPSettlement / 3)
		_, _ = peer.Write(append([]byte(`{"jsonrpc":"2.0","method":"progress","params":{"extension":"`),
			append([]byte{0xff}, []byte(`"}}`+"\n")...)...))
	}()
	err = exchangeKiroACP(stdin, stdout, t.TempDir(), map[string]*kiroACPServerState{"alpha": {}})
	if err == nil || !strings.Contains(err.Error(), "valid UTF-8") {
		t.Fatalf("settlement error = %v, want queued invalid UTF-8 rejected", err)
	}
}

func TestDecodeACPMessageWideObjectRemainsResourceBounded(t *testing.T) {
	t.Parallel()
	var record strings.Builder
	record.WriteString(`{"jsonrpc":"2.0","method":"progress","params":{"wide":{`)
	for index := 0; index < 12000; index++ {
		if index != 0 {
			record.WriteByte(',')
		}
		record.WriteString(strconv.Quote(fmt.Sprintf("k%05d", index)))
		record.WriteString(":0")
	}
	record.WriteString(`}}}`)
	if record.Len() >= kiroACPMaxLine {
		t.Fatalf("wide-object fixture exceeds legal ACP line width: %d", record.Len())
	}
	started := time.Now()
	if _, err := decodeACPMessage([]byte(record.String())); err != nil {
		t.Fatal(err)
	}
	if elapsed := time.Since(started); elapsed > 3*time.Second {
		t.Fatalf("legal-width object decoding was not linearly resource-bounded: %s", elapsed)
	}
}

func TestKiroACPPreservesPreSessionNegativeEvidenceAcrossLaterFailures(t *testing.T) {
	t.Parallel()
	prefix := acpResponse(0, `{"protocolVersion":1}`) + acpStatus("s", "alpha", "disconnected", "")
	readFailure := errors.New("deterministic ACP read failure")
	tests := map[string]io.Reader{
		"malformed": strings.NewReader(prefix + "not-json\n"),
		"unknown":   strings.NewReader(prefix + acpStatus("s", "alpha", "future-state", "")),
		"byte cap":  strings.NewReader(prefix + strings.Repeat("x", kiroACPMaxLine+1) + "\n"),
		"EOF":       strings.NewReader(prefix),
		"read":      io.MultiReader(strings.NewReader(prefix), terminalErrorReader{err: readFailure}),
	}
	for name, output := range tests {
		output := output
		t.Run(name, func(t *testing.T) {
			runner := &fixtureDuplexRunner{reader: output}
			err := VerifyACP(context.Background(), runner, "kiro-cli", t.TempDir(), []string{"alpha"})
			if !errors.Is(err, shared.ErrRecognizedNegativeEvidence) || errors.Is(err, ErrACPContractUnknown) {
				t.Fatalf("error = %v, want sticky recognized negative evidence", err)
			}
		})
	}
}

func TestKiroACPMultiServerTimeoutPreservesIncompleteNegativeEvidence(t *testing.T) {
	t.Parallel()
	runner := &fixtureDuplexRunner{
		output: acpResponse(0, `{"protocolVersion":1}`) + acpResponse(1, `{"sessionId":"s"}`) +
			acpStatus("s", "alpha", "connected", `[{"name":"search","disabled":false}]`),
		postError: context.DeadlineExceeded,
	}
	err := VerifyACP(context.Background(), runner, "kiro-cli", t.TempDir(), []string{"alpha", "beta"})
	if !errors.Is(err, shared.ErrRecognizedNegativeEvidence) || !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("error = %v, want incomplete evidence and deadline identities", err)
	}
}

func TestKiroACPPropagatesCallerCancellation(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	runner := &fixtureDuplexRunner{}
	err := VerifyACP(ctx, runner, "kiro-cli", t.TempDir(), []string{"alpha"})
	if !errors.Is(err, context.Canceled) || errors.Is(err, ErrACPContractUnknown) {
		t.Fatalf("error = %v, want caller cancellation without manual fallback", err)
	}
	if len(runner.command.Argv) != 0 {
		t.Fatalf("canceled verification unexpectedly launched %+v", runner.command)
	}
}

func TestKiroACPRejectsNonExclusiveJSONRPCEnvelopes(t *testing.T) {
	t.Parallel()
	validInitialize := acpResponse(0, `{"protocolVersion":1}`)
	tests := map[string]string{
		"initialize method and id":     `{"jsonrpc":"2.0","id":0,"method":"initialize","result":{"protocolVersion":1}}` + "\n",
		"initialize result and error":  `{"jsonrpc":"2.0","id":0,"result":{"protocolVersion":1},"error":{"code":-1}}` + "\n",
		"initialize null error":        `{"jsonrpc":"2.0","id":0,"error":null}` + "\n",
		"initialize result and params": `{"jsonrpc":"2.0","id":0,"result":{"protocolVersion":1},"params":{}}` + "\n",
		"session method and id":        validInitialize + `{"jsonrpc":"2.0","id":1,"method":"session/new","result":{"sessionId":"s"}}` + "\n",
		"session result and error":     validInitialize + `{"jsonrpc":"2.0","id":1,"result":{"sessionId":"s"},"error":{"code":-1}}` + "\n",
		"session null error":           validInitialize + `{"jsonrpc":"2.0","id":1,"error":null}` + "\n",
		"session result and params":    validInitialize + `{"jsonrpc":"2.0","id":1,"result":{"sessionId":"s"},"params":{}}` + "\n",
	}
	for name, output := range tests {
		output := output
		t.Run(name, func(t *testing.T) {
			runner := &fixtureDuplexRunner{output: output}
			err := VerifyACP(context.Background(), runner, "kiro-cli", t.TempDir(), []string{"alpha"})
			if !errors.Is(err, shared.ErrRecognizedNegativeEvidence) || errors.Is(err, ErrACPContractUnknown) {
				t.Fatalf("error = %v, want rejected JSON-RPC protocol failure", err)
			}
		})
	}
}

func TestKiroACPRejectsStructuredNegativeEvidence(t *testing.T) {
	t.Parallel()
	tests := map[string]string{
		"disconnected":  acpStatus("s", "alpha", "disconnected", ""),
		"disabled":      `{"jsonrpc":"2.0","method":"_kiro/mcp/status","params":{"sessionId":"s","serverName":"alpha","status":"connected","disabled":true,"tools":[{"name":"search","disabled":false}]}}` + "\n",
		"error":         acpStatus("s", "alpha", "error", ""),
		"auth required": acpStatus("s", "alpha", "auth-required", ""),
		"empty tools":   acpStatus("s", "alpha", "connected", `[]`),
		"missing tools": `{"jsonrpc":"2.0","method":"_kiro/mcp/status","params":{"sessionId":"s","serverName":"alpha","status":"connected"}}` + "\n",
		"disabled tool": acpStatus("s", "alpha", "connected", `[{"name":"search","disabled":true}]`),
	}
	for name, status := range tests {
		status := status
		t.Run(name, func(t *testing.T) {
			runner := &fixtureDuplexRunner{output: acpResponse(0, `{"protocolVersion":1}`) + acpResponse(1, `{"sessionId":"s"}`) + status}
			err := VerifyACP(context.Background(), runner, "kiro-cli", t.TempDir(), []string{"alpha"})
			if !errors.Is(err, shared.ErrRecognizedNegativeEvidence) {
				t.Fatalf("error = %v, want recognized negative evidence", err)
			}
		})
	}
}

func TestKiroACPRejectsAmbiguousDuplicateAndWrongIdentity(t *testing.T) {
	t.Parallel()
	connected := acpStatus("s", "alpha", "connected", `[{"name":"search","disabled":false}]`)
	tests := map[string]string{
		"conflicting identity": acpResponse(0, `{"protocolVersion":1}`) + `{"jsonrpc":"2.0","method":"_kiro/mcp/status","params":{"sessionId":"s","serverName":"alpha","name":"beta","status":"connected","tools":[{"name":"search","disabled":false}]}}` + "\n",
		"duplicate identity":   acpResponse(0, `{"protocolVersion":1}`) + `{"jsonrpc":"2.0","method":"_kiro/mcp/status","params":{"sessionId":"s","serverName":"alpha","name":"alpha","status":"connected","tools":[{"name":"search","disabled":false}]}}` + "\n",
		"missing identity":     acpResponse(0, `{"protocolVersion":1}`) + `{"jsonrpc":"2.0","method":"_kiro/mcp/status","params":{"sessionId":"s","status":"connected","tools":[{"name":"search","disabled":false}]}}` + "\n",
		"conflicting duplicate connected": acpResponse(0, `{"protocolVersion":1}`) + connected +
			acpStatus("s", "alpha", "connected", `[{"name":"different","disabled":false}]`) + acpResponse(1, `{"sessionId":"s"}`),
		"wrong server":   acpResponse(0, `{"protocolVersion":1}`) + acpResponse(1, `{"sessionId":"s"}`) + acpStatus("s", "beta", "connected", `[{"name":"search","disabled":false}]`),
		"missing server": acpResponse(0, `{"protocolVersion":1}`) + acpResponse(1, `{"sessionId":"s"}`),
		"wrong session":  acpResponse(0, `{"protocolVersion":1}`) + acpResponse(1, `{"sessionId":"s"}`) + acpStatus("other", "alpha", "connected", `[{"name":"search","disabled":false}]`),
	}
	for name, output := range tests {
		output := output
		t.Run(name, func(t *testing.T) {
			runner := &fixtureDuplexRunner{output: output}
			err := VerifyACP(context.Background(), runner, "kiro-cli", t.TempDir(), []string{"alpha"})
			if err == nil {
				t.Fatal("ambiguous ACP evidence unexpectedly succeeded")
			}
			if name == "wrong server" || name == "missing server" || name == "wrong session" || name == "conflicting duplicate connected" {
				if !errors.Is(err, shared.ErrRecognizedNegativeEvidence) {
					t.Fatalf("error = %v, want recognized negative evidence", err)
				}
			}
		})
	}
}

func TestKiroACPProtocolFailuresFailClosedAndLaunchFailuresFallBack(t *testing.T) {
	t.Parallel()
	progress := `{"jsonrpc":"2.0","method":"progress","params":{}}` + "\n"
	paddedProgress := `{"jsonrpc":"2.0","method":"progress","params":{"padding":"` + strings.Repeat("x", 200*1024) + `"}}` + "\n"
	tests := map[string]struct {
		output string
		err    error
	}{
		"malformed JSON":           {output: "not-json\n"},
		"wrong protocol":           {output: acpResponse(0, `{"protocolVersion":2}`)},
		"wrong initialize id":      {output: acpResponse(9, `{"protocolVersion":1}`)},
		"wrong session id":         {output: acpResponse(0, `{"protocolVersion":1}`) + acpResponse(2, `{"sessionId":"s"}`)},
		"missing session identity": {output: acpResponse(0, `{"protocolVersion":1}`) + acpResponse(1, `{}`)},
		"oversized output":         {output: strings.Repeat("x", kiroACPMaxLine+1) + "\n"},
		"total byte overflow":      {output: acpResponse(0, `{"protocolVersion":1}`) + strings.Repeat(paddedProgress, 6)},
		"record overflow":          {output: acpResponse(0, `{"protocolVersion":1}`) + strings.Repeat(progress, kiroACPMaxRecords)},
		"early exit":               {output: acpResponse(0, `{"protocolVersion":1}`)},
		"missing companion":        {err: errors.New("fork/exec kiro-cli-chat: executable file not found")},
		"ACP unsupported":          {err: errors.New("unknown command acp")},
		"timeout":                  {err: context.DeadlineExceeded},
	}
	for name, test := range tests {
		test := test
		t.Run(name, func(t *testing.T) {
			runner := &fixtureDuplexRunner{output: test.output, duplexErr: test.err}
			err := VerifyACP(context.Background(), runner, "kiro-cli", t.TempDir(), []string{"alpha"})
			if test.err == nil {
				if !errors.Is(err, shared.ErrRecognizedNegativeEvidence) || errors.Is(err, ErrACPContractUnknown) {
					t.Fatalf("error = %v, want fail-closed protocol evidence", err)
				}
			} else if !errors.Is(err, ErrACPContractUnknown) || errors.Is(err, shared.ErrRecognizedNegativeEvidence) {
				t.Fatalf("error = %v, want safe pre-exchange fallback", err)
			}
		})
	}
}
