package kiro

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/ports"
	legacyports "github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

const (
	kiroACPTimeout    = 40 * time.Second
	kiroACPMaxBytes   = 1024 * 1024
	kiroACPMaxLine    = 256 * 1024
	kiroACPMaxRecords = 256
	kiroACPSettlement = 150 * time.Millisecond
)

var (
	ErrACPContractUnknown = kiroError("Kiro structured ACP verification is unavailable or unrecognized")
	errKiroACPEarlyExit   = kiroError("Kiro ACP process exited before verification completed")
	ErrACPPartialExit     = fmt.Errorf("%w: Kiro ACP process exited with a partial trailing record", shared.ErrRecognizedNegativeEvidence)
)

type kiroACPServerState struct {
	connecting       bool
	connected        bool
	sessionID        string
	connectingRecord string
	connectedRecord  string
}

type acpMessage struct {
	document map[string]any
}

type acpDeadlineWriter interface {
	SetReadDeadline(time.Time) error
}

// VerifyACP performs only ACP initialization and session creation. The
// empty mcpServers list is intentional: Kiro must discover the native config
// installed by the preceding import rather than receive package definitions
// through the verification channel.
func VerifyACP(ctx context.Context, runner ports.DuplexCommandRunner, executable, cwd string, servers []string) error {
	return verifyACPWithTimeout(ctx, runner, executable, cwd, servers, kiroACPTimeout)
}

func verifyACPWithTimeout(ctx context.Context, runner ports.DuplexCommandRunner, executable, cwd string, servers []string, startupTimeout time.Duration) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if runner == nil {
		return fmt.Errorf("%w: interactive process runner is unavailable", ErrACPContractUnknown)
	}
	absoluteCWD, err := filepath.Abs(cwd)
	if err != nil || !filepath.IsAbs(absoluteCWD) {
		return fmt.Errorf("%w: resolve prepared verification directory", ErrACPContractUnknown)
	}
	expected, err := expectedACPServers(servers)
	if err != nil {
		return err
	}
	if startupTimeout <= 0 {
		return fmt.Errorf("%w: ACP startup timeout must be positive", ErrACPContractUnknown)
	}
	bounded, cancel := context.WithTimeout(ctx, startupTimeout)
	defer cancel()
	command := legacyports.Command{
		Argv: []string{executable, "acp", "--agent-engine", "v3", "--auth-method", "cli"},
		Dir:  absoluteCWD,
	}
	var exchangeEntered atomic.Bool
	err = runner.RunDuplexWithPlannedShutdown(bounded, command, func(stdin io.Writer, stdout io.Reader) error {
		exchangeEntered.Store(true)
		exchangeErr := exchangeKiroACP(stdin, stdout, absoluteCWD, expected)
		if exchangeErr == nil {
			return nil
		}
		// Once the client has entered the ACP exchange, malformed records,
		// partial output, protocol violations, and stream/process interruption
		// are authoritative verification failures rather than fallback signals.
		return errors.Join(shared.ErrRecognizedNegativeEvidence, exchangeErr)
	})
	return classifyACPRunnerError(ctx, bounded, err, exchangeEntered.Load())
}

func expectedACPServers(servers []string) (map[string]*kiroACPServerState, error) {
	expected := make(map[string]*kiroACPServerState, len(servers))
	for _, rawName := range servers {
		name := strings.TrimSpace(rawName)
		if name == "" {
			return nil, fmt.Errorf("%w: planned MCP server name is empty", ErrACPContractUnknown)
		}
		if _, duplicate := expected[name]; duplicate {
			return nil, fmt.Errorf("%w: duplicate planned MCP server identity %q", ErrACPContractUnknown, name)
		}
		expected[name] = &kiroACPServerState{}
	}
	if len(expected) == 0 {
		return nil, fmt.Errorf("%w: no planned MCP servers", ErrACPContractUnknown)
	}
	return expected, nil
}

func classifyACPRunnerError(ctx, bounded context.Context, err error, exchangeEntered bool) error {
	if err == nil {
		if ctxErr := bounded.Err(); ctxErr != nil {
			return errors.Join(shared.ErrRecognizedNegativeEvidence, ctxErr)
		}
		return nil
	}
	if exchangeEntered {
		if !errors.Is(err, shared.ErrRecognizedNegativeEvidence) {
			err = errors.Join(shared.ErrRecognizedNegativeEvidence, err)
		}
		// The phase-specific deadline belongs to the ACP exchange even when pipe
		// closure is observed first. Preserve that cause instead of degrading a
		// no-data timeout into an indistinguishable early EOF.
		if boundedErr := bounded.Err(); boundedErr != nil && !errors.Is(err, boundedErr) {
			err = errors.Join(err, boundedErr)
		}
	}
	if errors.Is(err, shared.ErrRecognizedNegativeEvidence) {
		if ctxErr := ctx.Err(); ctxErr != nil && !errors.Is(err, ctxErr) {
			return errors.Join(err, ctxErr)
		}
		return err
	}
	if ctxErr := ctx.Err(); ctxErr != nil {
		return ctxErr
	}
	// Keep the runner cause out of the unwrap chain so errors.Is(err,
	// ErrACPContractUnknown) stays the contract-unknown signal.
	return fmt.Errorf("%w: %s", ErrACPContractUnknown, err.Error())
}

func exchangeKiroACP(stdin io.Writer, stdout io.Reader, cwd string, expected map[string]*kiroACPServerState) error {
	if _, ok := stdin.(io.Closer); !ok {
		return fmt.Errorf("ACP stdin does not support close")
	}
	exchange := &acpExchange{stdin: stdin, stdout: stdout, cwd: cwd, expected: expected}
	return exchange.run()
}

type acpExchange struct {
	stdin          io.Writer
	stdout         io.Reader
	cwd            string
	expected       map[string]*kiroACPServerState
	initialized    bool
	sessionCreated bool
	sessionID      string
	negativeErr    error
	settling       bool
}

func (exchange *acpExchange) run() error {
	initialize := map[string]any{
		"jsonrpc": "2.0", "id": 0, "method": "initialize",
		"params": map[string]any{
			"protocolVersion":    1,
			"clientCapabilities": map[string]any{},
			"clientInfo":         map[string]any{"name": "agentplugins", "version": "1"},
		},
	}
	if err := writeACPRecord(exchange.stdin, initialize); err != nil {
		return fmt.Errorf("write initialize request: %w", err)
	}
	reader := bufio.NewReaderSize(exchange.stdout, kiroACPMaxLine+1)
	totalBytes := 0
	for record := 0; ; record++ {
		line, err := readBoundedACPLine(reader, &totalBytes)
		if err != nil {
			retry, readErr := interpretACPReadError(err, exchange.stdout, exchange.expected, exchange.sessionID, exchange.sessionCreated, exchange.settling, exchange.negativeErr)
			if !retry {
				return readErr
			}
			continue
		}
		if record >= kiroACPMaxRecords {
			return fmt.Errorf("ACP output exceeded its record bound")
		}
		if err := exchange.handleLine(line); err != nil {
			return err
		}
	}
}

func (exchange *acpExchange) handleLine(line []byte) error {
	message, err := decodeACPMessage(line)
	if err != nil {
		if exchange.negativeErr != nil {
			return exchange.negativeErr
		}
		return err
	}
	if !exchange.initialized {
		if err := completeACPInitialize(exchange.stdin, exchange.cwd, message); err != nil {
			return err
		}
		exchange.initialized = true
		return nil
	}
	if err := exchange.consumeRecord(message); err != nil {
		return err
	}
	if exchange.sessionCreated && exchange.negativeErr != nil {
		return exchange.negativeErr
	}
	if exchange.sessionCreated && allKiroServersConnected(exchange.expected, exchange.sessionID) {
		if _, err := armACPSettlement(exchange.stdout, &exchange.settling); err != nil {
			return err
		}
	}
	return nil
}

func interpretACPReadError(err error, stdout io.Reader, expected map[string]*kiroACPServerState, sessionID string, sessionCreated, settling bool, negativeErr error) (retry bool, result error) {
	if negativeErr != nil {
		return false, negativeErr
	}
	if settling && errors.Is(err, os.ErrDeadlineExceeded) {
		queued, eof, probeErr := queuedACPRealPipeEvidence(stdout)
		if probeErr != nil {
			return false, fmt.Errorf("inspect ACP pipe at settlement boundary: %w", probeErr)
		}
		if eof {
			return false, fmt.Errorf("%w: ACP output reached EOF at the post-success settlement boundary", shared.ErrRecognizedNegativeEvidence)
		}
		if !queued {
			return false, nil
		}
		deadlineWriter := stdout.(acpDeadlineWriter)
		if err := deadlineWriter.SetReadDeadline(time.Now().Add(kiroACPSettlement)); err != nil {
			return false, fmt.Errorf("rearm ACP queued-evidence drain deadline: %w", err)
		}
		return true, nil
	}
	if settling && errors.Is(err, errKiroACPEarlyExit) {
		return false, fmt.Errorf("%w: ACP output reached EOF during post-success settlement", shared.ErrRecognizedNegativeEvidence)
	}
	if sessionCreated && !allKiroServersConnected(expected, sessionID) && errors.Is(err, errKiroACPEarlyExit) {
		return false, fmt.Errorf("%w: not every planned MCP server reported connected", shared.ErrRecognizedNegativeEvidence)
	}
	return false, err
}

func completeACPInitialize(stdin io.Writer, cwd string, message acpMessage) error {
	if err := validateInitializeResponse(message); err != nil {
		return err
	}
	sessionRequest := map[string]any{
		"jsonrpc": "2.0", "id": 1, "method": "session/new",
		"params": map[string]any{"cwd": cwd, "mcpServers": []any{}},
	}
	if err := writeACPRecord(stdin, sessionRequest); err != nil {
		return fmt.Errorf("write session/new request: %w", err)
	}
	return nil
}

func (exchange *acpExchange) consumeRecord(message acpMessage) error {
	if _, hasID := message.document["id"]; hasID {
		return exchange.completeSession(message)
	}
	method, ok := message.document["method"].(string)
	if !ok || strings.TrimSpace(method) == "" {
		if exchange.negativeErr != nil {
			return exchange.negativeErr
		}
		return fmt.Errorf("ACP notification is missing its method")
	}
	if method != "_kiro/mcp/status" {
		return nil
	}
	err := consumeKiroMCPStatus(message.document, exchange.expected)
	if err == nil {
		return nil
	}
	if errors.Is(err, shared.ErrRecognizedNegativeEvidence) {
		if exchange.negativeErr == nil {
			exchange.negativeErr = err
		}
		return nil
	}
	if exchange.negativeErr != nil {
		return exchange.negativeErr
	}
	return err
}

func (exchange *acpExchange) completeSession(message acpMessage) error {
	if exchange.sessionCreated || !acpIDEquals(message.document["id"], 1) {
		if exchange.negativeErr != nil {
			return exchange.negativeErr
		}
		return fmt.Errorf("ACP response has an unexpected or duplicate id")
	}
	result, ok := message.document["result"].(map[string]any)
	_, hasError := message.document["error"]
	if !ok || hasError {
		if exchange.negativeErr != nil {
			return exchange.negativeErr
		}
		return fmt.Errorf("session/new did not return a valid result")
	}
	id, okSession := result["sessionId"].(string)
	if !okSession || strings.TrimSpace(id) == "" {
		if exchange.negativeErr != nil {
			return exchange.negativeErr
		}
		return fmt.Errorf("session/new response is missing its session identity")
	}
	exchange.sessionID = id
	exchange.sessionCreated = true
	for _, state := range exchange.expected {
		if state.sessionID != "" && state.sessionID != id {
			return fmt.Errorf("%w: MCP status belongs to a different ACP session", shared.ErrRecognizedNegativeEvidence)
		}
	}
	return nil
}

func armACPSettlement(stdout io.Reader, settling *bool) (armed bool, err error) {
	if *settling {
		return true, nil
	}
	deadlineWriter, ok := stdout.(acpDeadlineWriter)
	if !ok {
		// Finite readers used by callers/tests still provide EOF as the
		// completion boundary. Real process pipes always support deadlines.
		return false, nil
	}
	if err := deadlineWriter.SetReadDeadline(time.Now().Add(kiroACPSettlement)); err != nil {
		return false, fmt.Errorf("arm ACP post-success settlement deadline: %w", err)
	}
	*settling = true
	return true, nil
}
