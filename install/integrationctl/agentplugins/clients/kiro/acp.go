package kiro

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
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
	ErrACPContractUnknown = errors.New("the Kiro structured ACP verification is unavailable or unrecognized")
	errKiroACPEarlyExit   = errors.New("the Kiro ACP process exited before verification completed")
	ErrACPPartialExit     = fmt.Errorf("%w: Kiro ACP process exited with a partial trailing record", shared.ErrRecognizedNegativeEvidence)
)

type kiroACPServerState struct {
	connecting       bool
	connected        bool
	sessionID        string
	connectingRecord string
	connectedRecord  string
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
	expected, absoluteCWD, err := prepareKiroACPExchange(runner, cwd, servers, startupTimeout)
	if err != nil {
		return err
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
		return errors.Join(shared.ErrRecognizedNegativeEvidence, exchangeErr)
	})
	return interpretKiroACPRun(ctx, bounded, exchangeEntered.Load(), err)
}

func prepareKiroACPExchange(runner ports.DuplexCommandRunner, cwd string, servers []string, startupTimeout time.Duration) (map[string]*kiroACPServerState, string, error) {
	if runner == nil {
		return nil, "", fmt.Errorf("%w: interactive process runner is unavailable", ErrACPContractUnknown)
	}
	absoluteCWD, err := filepath.Abs(cwd)
	if err != nil || !filepath.IsAbs(absoluteCWD) {
		return nil, "", fmt.Errorf("%w: resolve prepared verification directory", ErrACPContractUnknown)
	}
	expected := make(map[string]*kiroACPServerState, len(servers))
	for _, rawName := range servers {
		name := strings.TrimSpace(rawName)
		if name == "" {
			return nil, "", fmt.Errorf("%w: planned MCP server name is empty", ErrACPContractUnknown)
		}
		if _, duplicate := expected[name]; duplicate {
			return nil, "", fmt.Errorf("%w: duplicate planned MCP server identity %q", ErrACPContractUnknown, name)
		}
		expected[name] = &kiroACPServerState{}
	}
	if len(expected) == 0 {
		return nil, "", fmt.Errorf("%w: no planned MCP servers", ErrACPContractUnknown)
	}
	if startupTimeout <= 0 {
		return nil, "", fmt.Errorf("%w: ACP startup timeout must be positive", ErrACPContractUnknown)
	}
	return expected, absoluteCWD, nil
}

func interpretKiroACPRun(ctx context.Context, bounded context.Context, exchangeEntered bool, err error) error {
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
	return fmt.Errorf("%w: %w", ErrACPContractUnknown, err)
}

func exchangeKiroACP(stdin io.Writer, stdout io.Reader, cwd string, expected map[string]*kiroACPServerState) error {
	if _, ok := stdin.(io.Closer); !ok {
		return fmt.Errorf("ACP stdin does not support close")
	}
	initialize := map[string]any{
		"jsonrpc": "2.0", "id": 0, "method": "initialize",
		"params": map[string]any{
			"protocolVersion":    1,
			"clientCapabilities": map[string]any{},
			"clientInfo":         map[string]any{"name": "agentplugins", "version": "1"},
		},
	}
	if err := writeACPRecord(stdin, initialize); err != nil {
		return fmt.Errorf("write initialize request: %w", err)
	}
	session := &kiroACPSession{
		stdin: stdin, stdout: stdout, cwd: cwd, expected: expected,
		reader: bufio.NewReaderSize(stdout, kiroACPMaxLine+1),
	}
	return session.readLoop()
}

type acpMessage struct {
	document map[string]any
}
