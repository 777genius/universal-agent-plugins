package pluginkitairepo_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/claude"
)

const claudeDiagnosticLimit = 1 << 20

// Writes are capped while exec copies each pipe; overflow cancels the direct
// read-only child. WaitDelay bounds inherited pipes, not a process sandbox.
type claudeDiagnosticStream struct {
	mu       sync.Mutex
	body     []byte
	limit    int
	overflow bool
	cancel   context.CancelFunc
}

func (s *claudeDiagnosticStream) Write(body []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	n := len(body)
	if remaining := s.limit - len(s.body); n > remaining {
		n = remaining
		s.overflow = true
	}
	s.body = append(s.body, body[:n]...)
	if s.overflow {
		s.cancel()
		return n, errors.New("Claude diagnostic output limit exceeded")
	}
	return n, nil
}

func (s *claudeDiagnosticStream) snapshot() ([]byte, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]byte(nil), s.body...), s.overflow
}

type claudeDiagnosticResult struct {
	Stdout, Stderr                     []byte
	Exit                               int
	Error                              string
	Deadline, StdoutLimit, StderrLimit bool
	Elapsed                            time.Duration
}

type claudeDiagnosticCapture func(string, string, []string, []string) claudeDiagnosticResult

func captureClaudeDiagnostic(client, cwd string, env, args []string) claudeDiagnosticResult {
	return captureClaudeDiagnosticInput(client, cwd, env, args, nil)
}

func captureClaudeDiagnosticInput(client, cwd string, env, args []string, input []byte) claudeDiagnosticResult {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	stdout := &claudeDiagnosticStream{limit: claudeDiagnosticLimit, cancel: cancel}
	stderr := &claudeDiagnosticStream{limit: claudeDiagnosticLimit, cancel: cancel}
	command := exec.CommandContext(ctx, client, args...)
	command.Dir, command.Env = cwd, append([]string(nil), env...)
	command.Stdin = bytes.NewReader(input)
	command.Stdout, command.Stderr = stdout, stderr
	command.WaitDelay = 250 * time.Millisecond
	started := time.Now()
	err := command.Run()
	out := claudeDiagnosticResult{Exit: -1, Elapsed: time.Since(started), Deadline: errors.Is(ctx.Err(), context.DeadlineExceeded)}
	out.Stdout, out.StdoutLimit = stdout.snapshot()
	out.Stderr, out.StderrLimit = stderr.snapshot()
	if command.ProcessState != nil {
		out.Exit = command.ProcessState.ExitCode()
	}
	if err != nil {
		out.Error = boundedClaudeDiagnosticError(err)
	}
	return out
}

func boundedClaudeDiagnosticError(err error) string {
	value := err.Error()
	if len(value) > 512 {
		return value[:512]
	}
	return value
}

func claudeFailureListingRecords(client, project, other string, env []string, capture claudeDiagnosticCapture) ([]map[string]any, map[string][]byte) {
	records := make([]map[string]any, 0, 2)
	logs := map[string][]byte{}
	for _, selected := range []struct{ label, cwd string }{{"failure-project-A-list", project}, {"failure-project-B-list", other}} {
		args := []string{"plugin", "list", "--json"}
		result := capture(client, selected.cwd, append([]string(nil), env...), args)
		record, streams := claudeDiagnosticRecord(selected.label, client, selected.cwd, args, result)
		records = append(records, record)
		for name, body := range streams {
			logs[name] = body
		}
	}
	return records, logs
}

func claudeDiagnosticRecord(label, client, cwd string, args []string, result claudeDiagnosticResult) (map[string]any, map[string][]byte) {
	stdout, stderr := label+".stdout.log", label+".stderr.log"
	digest := func(body []byte) string { sum := sha256.Sum256(body); return hex.EncodeToString(sum[:]) }
	record := map[string]any{"argv": append([]string{client}, args...), "cwd": cwd, "exit": result.Exit, "error": result.Error, "deadline": result.Deadline, "stdout_limit": result.StdoutLimit, "stderr_limit": result.StderrLimit, "elapsed_ms": result.Elapsed.Milliseconds(), "stdout_file": stdout, "stderr_file": stderr, "stdout_bytes": len(result.Stdout), "stderr_bytes": len(result.Stderr), "stdout_sha256": digest(result.Stdout), "stderr_sha256": digest(result.Stderr), "qualification": false}
	return record, map[string][]byte{stdout: result.Stdout, stderr: result.Stderr}
}

// This is a later diagnostic observation, not the missing original activation
// response. It cannot qualify the failed installer or a native MCP session.
func claudeFailureMCPRecord(label, client, project, configRoot string, originalEnv []string, capture claudeDiagnosticCapture) (map[string]any, map[string][]byte) {
	if label != "managed-mcp-add" {
		return nil, nil
	}
	args := []string{"mcp", "get", "plugin:uap-commands-fixture:uap_project_fixture"}
	env, err := claudeFailureMCPEnvironment(configRoot, originalEnv)
	result := claudeDiagnosticResult{Exit: -1}
	if err != nil {
		result.Error = boundedClaudeDiagnosticError(err)
	} else {
		result = capture(client, project, env, args)
	}
	record, logs := claudeDiagnosticRecord("failure-project-A-mcp-get", client, project, args, result)
	record["post_failure"], record["capture_attempted"] = true, err == nil
	record["config_root"] = configRoot
	return record, logs
}

func claudeFailureMCPEnvironment(configRoot string, original []string) ([]string, error) {
	if !filepath.IsAbs(configRoot) || len(configRoot) > 4096 || strings.ContainsRune(configRoot, '\x00') {
		return nil, errors.New("invalid original Claude diagnostic config root")
	}
	env, _, err := claude.BoundedClaudeProbeEnvironmentFrom(original)
	if err != nil {
		return nil, err
	}
	env = append(env, "CLAUDE_CONFIG_DIR="+configRoot, "DISABLE_AUTOUPDATER=1", "CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC=1")
	total := 0
	for _, entry := range env {
		total += len(entry) + 1
	}
	if total > 32768 {
		return nil, errors.New("claude diagnostic environment exceeds the bounded size")
	}
	return env, nil
}
