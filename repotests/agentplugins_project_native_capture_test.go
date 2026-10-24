package pluginkitairepo_test

import (
	"context"
	"errors"
	"os/exec"
	"sync"
	"time"
)

// These are the reviewed diagnostic capture mechanics with explicit limits.
// Direct-child cancellation and WaitDelay do not constitute a process sandbox.
type projectNativeStream struct {
	mu       sync.Mutex
	body     []byte
	limit    int
	overflow bool
	cancel   context.CancelFunc
}

func (s *projectNativeStream) Write(body []byte) (int, error) {
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
		return n, errors.New("project native output limit exceeded")
	}
	return n, nil
}
func (s *projectNativeStream) snapshot() ([]byte, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]byte(nil), s.body...), s.overflow
}

type projectNativeCaptureResult struct {
	Stdout      []byte `json:"-"`
	Stderr      []byte `json:"-"`
	Exit        int    `json:"exit"`
	Error       string `json:"error,omitempty"`
	ErrorKind   string `json:"error_kind,omitempty"`
	Deadline    bool   `json:"deadline"`
	StdoutLimit bool   `json:"stdout_limit"`
	StderrLimit bool   `json:"stderr_limit"`
	ElapsedMS   int64  `json:"elapsed_ms"`
}

func captureProjectNative(parent context.Context, executable, cwd string, env, args []string, limit int, timeout time.Duration) projectNativeCaptureResult {
	ctx, cancel := context.WithTimeout(parent, timeout)
	defer cancel()
	stdout := &projectNativeStream{limit: limit, cancel: cancel}
	stderr := &projectNativeStream{limit: limit, cancel: cancel}
	command := exec.CommandContext(ctx, executable, args...)
	command.Dir = cwd
	command.Env = append([]string(nil), env...)
	command.Stdin = nil
	command.Stdout, command.Stderr = stdout, stderr
	command.WaitDelay = 250 * time.Millisecond
	start := time.Now()
	err := command.Run()
	result := projectNativeCaptureResult{Exit: -1, ElapsedMS: time.Since(start).Milliseconds(), Deadline: errors.Is(ctx.Err(), context.DeadlineExceeded)}
	result.Stdout, result.StdoutLimit = stdout.snapshot()
	result.Stderr, result.StderrLimit = stderr.snapshot()
	if command.ProcessState != nil {
		result.Exit = command.ProcessState.ExitCode()
	}
	if err != nil {
		result.ErrorKind = projectNativeErrorKind(err, ctx.Err())
		result.Error = err.Error()
		if len(result.Error) > 512 {
			result.Error = result.Error[:512]
		}
	}
	return result
}

func projectNativeErrorKind(err, contextErr error) string {
	if contextErr != nil {
		return "context"
	}
	if err == nil {
		return ""
	}
	if _, ok := err.(*exec.ExitError); ok {
		return "exit"
	}
	return "capture"
}
func projectNativeCaptureAdmission(result projectNativeCaptureResult, expectedAbsent bool) error {
	if result.Deadline || result.StdoutLimit || result.StderrLimit {
		return errors.New("capture deadline or stream limit")
	}
	if result.Error == "" && result.ErrorKind == "" && result.Exit == 0 {
		return nil
	}
	if expectedAbsent && result.Error != "" && result.ErrorKind == "exit" && result.Exit == 1 {
		return nil
	}
	return errors.New("process/capture did not complete cleanly")
}
