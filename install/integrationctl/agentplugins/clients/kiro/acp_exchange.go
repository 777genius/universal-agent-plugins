package kiro

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
)

type kiroACPSession struct {
	stdin          io.Writer
	stdout         io.Reader
	cwd            string
	expected       map[string]*kiroACPServerState
	reader         *bufio.Reader
	initialized    bool
	sessionCreated bool
	sessionID      string
	negativeErr    error
	settling       bool
	totalBytes     int
}

func (session *kiroACPSession) readLoop() error {
	for record := 0; ; record++ {
		line, err := readBoundedACPLine(session.reader, &session.totalBytes)
		if err != nil {
			cont, result := session.handleReadError(err)
			if result != nil {
				return result
			}
			if cont {
				continue
			}
			return nil
		}
		if record >= kiroACPMaxRecords {
			return fmt.Errorf("ACP output exceeded its record bound")
		}
		message, err := decodeACPMessage(line)
		if err != nil {
			if session.negativeErr != nil {
				return session.negativeErr
			}
			return err
		}
		if err := session.handleMessage(message); err != nil {
			return err
		}
		if session.sessionCreated && session.negativeErr != nil {
			return session.negativeErr
		}
		if err := session.maybeSettle(); err != nil {
			return err
		}
	}
}

func (session *kiroACPSession) handleReadError(err error) (continueLoop bool, result error) {
	if session.negativeErr != nil {
		return false, session.negativeErr
	}
	if session.settling && errors.Is(err, os.ErrDeadlineExceeded) {
		queued, eof, probeErr := queuedACPRealPipeEvidence(session.stdout)
		if probeErr != nil {
			return false, fmt.Errorf("inspect ACP pipe at settlement boundary: %w", probeErr)
		}
		if eof {
			return false, fmt.Errorf("%w: ACP output reached EOF at the post-success settlement boundary", shared.ErrRecognizedNegativeEvidence)
		}
		if !queued {
			return false, nil
		}
		deadlineWriter := session.stdout.(interface{ SetReadDeadline(time.Time) error })
		if err := deadlineWriter.SetReadDeadline(time.Now().Add(kiroACPSettlement)); err != nil {
			return false, fmt.Errorf("rearm ACP queued-evidence drain deadline: %w", err)
		}
		return true, nil
	}
	if session.settling && errors.Is(err, errKiroACPEarlyExit) {
		return false, fmt.Errorf("%w: ACP output reached EOF during post-success settlement", shared.ErrRecognizedNegativeEvidence)
	}
	if session.sessionCreated && !allKiroServersConnected(session.expected, session.sessionID) && errors.Is(err, errKiroACPEarlyExit) {
		return false, fmt.Errorf("%w: not every planned MCP server reported connected", shared.ErrRecognizedNegativeEvidence)
	}
	return false, err
}

func (session *kiroACPSession) handleMessage(message acpMessage) error {
	if !session.initialized {
		return session.handleInitialize(message)
	}
	if _, hasID := message.document["id"]; hasID {
		return session.handleSessionResponse(message)
	}
	return session.handleNotification(message)
}

func (session *kiroACPSession) handleInitialize(message acpMessage) error {
	if err := validateInitializeResponse(message); err != nil {
		return err
	}
	session.initialized = true
	sessionRequest := map[string]any{
		"jsonrpc": "2.0", "id": 1, "method": "session/new",
		"params": map[string]any{"cwd": session.cwd, "mcpServers": []any{}},
	}
	if err := writeACPRecord(session.stdin, sessionRequest); err != nil {
		return fmt.Errorf("write session/new request: %w", err)
	}
	return nil
}

func (session *kiroACPSession) handleSessionResponse(message acpMessage) error {
	if session.sessionCreated || !acpIDEquals(message.document["id"], 1) {
		if session.negativeErr != nil {
			return session.negativeErr
		}
		return fmt.Errorf("ACP response has an unexpected or duplicate id")
	}
	result, ok := message.document["result"].(map[string]any)
	_, hasError := message.document["error"]
	if !ok || hasError {
		if session.negativeErr != nil {
			return session.negativeErr
		}
		return fmt.Errorf("session/new did not return a valid result")
	}
	var okSession bool
	session.sessionID, okSession = result["sessionId"].(string)
	if !okSession || strings.TrimSpace(session.sessionID) == "" {
		if session.negativeErr != nil {
			return session.negativeErr
		}
		return fmt.Errorf("session/new response is missing its session identity")
	}
	session.sessionCreated = true
	for _, state := range session.expected {
		if state.sessionID != "" && state.sessionID != session.sessionID {
			return fmt.Errorf("%w: MCP status belongs to a different ACP session", shared.ErrRecognizedNegativeEvidence)
		}
	}
	return nil
}

func (session *kiroACPSession) handleNotification(message acpMessage) error {
	method, ok := message.document["method"].(string)
	if !ok || strings.TrimSpace(method) == "" {
		if session.negativeErr != nil {
			return session.negativeErr
		}
		return fmt.Errorf("ACP notification is missing its method")
	}
	if method != "_kiro/mcp/status" {
		return nil
	}
	err := consumeKiroMCPStatus(message.document, session.expected)
	if err == nil {
		return nil
	}
	if errors.Is(err, shared.ErrRecognizedNegativeEvidence) {
		if session.negativeErr == nil {
			session.negativeErr = err
		}
		return nil
	}
	if session.negativeErr != nil {
		return session.negativeErr
	}
	return err
}

func (session *kiroACPSession) maybeSettle() error {
	if !session.sessionCreated || !allKiroServersConnected(session.expected, session.sessionID) || session.settling {
		return nil
	}
	deadlineWriter, ok := session.stdout.(interface{ SetReadDeadline(time.Time) error })
	if !ok {
		return nil
	}
	if err := deadlineWriter.SetReadDeadline(time.Now().Add(kiroACPSettlement)); err != nil {
		return fmt.Errorf("arm ACP post-success settlement deadline: %w", err)
	}
	session.settling = true
	return nil
}
