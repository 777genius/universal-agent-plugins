package pluginkitairepo_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os/exec"
	"sync"
	"time"
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
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	stdout := &claudeDiagnosticStream{limit: claudeDiagnosticLimit, cancel: cancel}
	stderr := &claudeDiagnosticStream{limit: claudeDiagnosticLimit, cancel: cancel}
	command := exec.CommandContext(ctx, client, args...)
	command.Dir, command.Env = cwd, append([]string(nil), env...)
	command.Stdin = nil
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
		stdout, stderr := selected.label+".stdout.log", selected.label+".stderr.log"
		logs[stdout], logs[stderr] = result.Stdout, result.Stderr
		digest := func(body []byte) string { sum := sha256.Sum256(body); return hex.EncodeToString(sum[:]) }
		records = append(records, map[string]any{"argv": append([]string{client}, args...), "cwd": selected.cwd, "exit": result.Exit, "error": result.Error, "deadline": result.Deadline, "stdout_limit": result.StdoutLimit, "stderr_limit": result.StderrLimit, "elapsed_ms": result.Elapsed.Milliseconds(), "stdout_file": stdout, "stderr_file": stderr, "stdout_bytes": len(result.Stdout), "stderr_bytes": len(result.Stderr), "stdout_sha256": digest(result.Stdout), "stderr_sha256": digest(result.Stderr), "qualification": false})
	}
	return records, logs
}
