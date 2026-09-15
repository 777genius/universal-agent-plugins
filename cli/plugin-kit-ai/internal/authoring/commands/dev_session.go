package commands

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"time"

	"github.com/777genius/plugin-kit-ai/cli/internal/authoring/mcpruntime"
	"github.com/777genius/plugin-kit-ai/cli/internal/authoring/project"
	"github.com/777genius/plugin-kit-ai/cli/internal/authoring/report"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/processlock"
)

const (
	devPollInterval = 100 * time.Millisecond
	devDebounce     = 250 * time.Millisecond
	devMaxCoalesce  = time.Second
)

func devSessionLockPath(cache, root string) (string, error) {
	if cache == "" || !filepath.IsAbs(cache) {
		return "", &mcpruntime.Error{Code: "runtime_dev_session_lock_unavailable"}
	}
	canonical, err := filepath.Abs(root)
	if err != nil {
		return "", &mcpruntime.Error{Code: "runtime_dev_session_lock_unavailable"}
	}
	canonical, err = filepath.EvalSymlinks(canonical)
	if err != nil {
		return "", &mcpruntime.Error{Code: "runtime_dev_session_lock_unavailable"}
	}
	digest := sha256.Sum256([]byte(filepath.Clean(canonical)))
	return filepath.Join(cache, "agentplugins", "author-dev-locks", hex.EncodeToString(digest[:])+".lock"), nil
}

func acquireDevSession(ctx context.Context, root string) (func() error, error) {
	cache, err := os.UserCacheDir()
	if err != nil {
		return nil, &mcpruntime.Error{Code: "runtime_dev_session_lock_unavailable"}
	}
	lockPath, err := devSessionLockPath(cache, root)
	if err != nil {
		return nil, err
	}
	release, err := (processlock.Lock{Path: lockPath}).Acquire(ctx)
	if err != nil {
		code := "runtime_dev_session_lock_unavailable"
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return nil, err
		}
		if errors.Is(err, processlock.ErrActive) {
			code = "runtime_dev_session_conflict"
		}
		return nil, &mcpruntime.Error{Code: code}
	}
	return release, nil
}

type devPending struct {
	project project.Result
	set     bool
	first   time.Time
	last    time.Time
}

type devFailurePending struct {
	project project.Result
	err     error
	code    string
	set     bool
	first   time.Time
	last    time.Time
}

func (pending *devFailurePending) observe(next project.Result, err error, code string, now time.Time) {
	pending.project = next
	pending.err = err
	if !pending.set {
		pending.set = true
		pending.first = now
		pending.last = now
	} else if pending.code != code {
		pending.last = now
	}
	pending.code = code
}

func (pending devFailurePending) ready(now time.Time) bool {
	return pending.set && (now.Sub(pending.last) >= devDebounce || now.Sub(pending.first) >= devMaxCoalesce)
}

func (pending *devFailurePending) take() (project.Result, error, string) {
	next, err, code := pending.project, pending.err, pending.code
	*pending = devFailurePending{}
	return next, err, code
}

func (pending *devPending) observe(next project.Result, now time.Time) {
	pending.project = next
	pending.last = now
	if !pending.set {
		pending.set = true
		pending.first = now
	}
}

func (pending devPending) ready(now time.Time) bool {
	return pending.set && (now.Sub(pending.last) >= devDebounce || now.Sub(pending.first) >= devMaxCoalesce)
}

func (pending *devPending) take() project.Result {
	next := pending.project
	*pending = devPending{}
	return next
}

func (a App) dev(ctx context.Context, req request) (r report.Report, err error) {
	type cycleResult struct {
		report report.Report
		err    error
	}
	release, lockErr := acquireDevSession(ctx, req.root)
	if lockErr != nil {
		r = report.New("dev", a.Revision)
		code := runtimeErrorCode(lockErr)
		r.SetRuntime("", false, false, false, 0, false, code)
		return r, lockErr
	}
	defer func() {
		if releaseErr := release(); releaseErr != nil {
			cleanupErr := &mcpruntime.Error{Code: "runtime_dev_session_cleanup_failed"}
			transport, initialized, listed, called, toolCount := "", false, false, false, 0
			if detail := r.RuntimeDetail; detail != nil {
				transport = detail.Transport
				initialized = detail.Initialize == report.Pass
				listed = detail.ListTools == report.Pass
				called = detail.ToolCall == report.Pass
				toolCount = detail.ToolCount
			}
			r.SetRuntime(transport, initialized, listed, called, toolCount, false, cleanupErr.Code)
			err = errors.Join(err, cleanupErr, releaseErr)
		}
	}()
	emit := func(next report.Report, cycleErr error) error {
		r = next
		if req.cycleOutput != nil {
			return req.cycleOutput(next, cycleErr)
		}
		return nil
	}
	readFailure := func(p project.Result, readErr error) (report.Report, error) {
		result := report.Build("dev", a.Revision, p, req.release)
		if readErr != nil {
			code, action := failure(readErr, "read")
			result.AddError(code, action)
			return result, readErr
		}
		identityErr := errors.New("runtime source identity unavailable")
		result.AddError("runtime_source_identity_unavailable", "Make the exact package tree readable and quiescent, then retry.")
		return result, identityErr
	}
	var cycle context.CancelFunc
	var done chan cycleResult
	stopCycle := func() (cycleResult, bool) {
		if cycle == nil {
			return cycleResult{}, false
		}
		cycle()
		result := <-done
		cycle = nil
		done = nil
		return result, true
	}
	// This defer is registered after the project-lock defer, so an active
	// runtime is always canceled and joined before the lock is released.
	defer func() {
		result, stopped := stopCycle()
		if stopped && result.err != nil && runtimeErrorCode(result.err) == "runtime_cleanup_failed" {
			err = errors.Join(err, result.err)
		}
	}()
	start := func(p project.Result) {
		cycleCtx, cancel := context.WithCancel(ctx)
		cycle = cancel
		resultChannel := make(chan cycleResult, 1)
		done = resultChannel
		run := req.runMCP
		if run == nil {
			run = mcpruntime.Run
		}
		go func(ch chan<- cycleResult) {
			result := report.Build("dev", a.Revision, p, req.release)
			evidence, runErr := run(cycleCtx, mcpruntime.Options{SourceRoot: req.root, Scratch: a.Projects.Scratch, Project: p, Server: req.server, Tool: req.tool, Fixture: req.fixture, AllowNetwork: req.allowNetwork, Deadline: req.deadline, Projects: a.Projects})
			result.SetRuntime(evidence.Transport, evidence.Initialize, evidence.ListTools, evidence.ToolCall, evidence.ToolCount, evidence.Cleanup, runtimeErrorCode(runErr))
			ch <- cycleResult{result, runErr}
		}(resultChannel)
	}
	p, readErr := a.Projects.Read(ctx, req.root)
	last := p.Input.Identity.TreeDigest
	failureCode := ""
	if readErr != nil || last == "" {
		failed, failedErr := readFailure(p, readErr)
		failureCode = runtimeErrorCode(failedErr)
		if failureCode == "runtime_failed" && failed.Error != nil {
			failureCode = failed.Error.Code
		}
		if outputErr := emit(failed, failedErr); outputErr != nil {
			return r, outputErr
		}
	} else {
		start(p)
	}
	ticker := time.NewTicker(devPollInterval)
	defer ticker.Stop()
	var pending devPending
	var failedPending devFailurePending
	for {
		select {
		case result := <-done:
			done = nil
			cycle = nil
			if outputErr := emit(result.report, result.err); outputErr != nil {
				return r, outputErr
			}
			if runtimeErrorCode(result.err) == "runtime_cleanup_failed" {
				return r, result.err
			}
		case <-ctx.Done():
			if result, stopped := stopCycle(); stopped {
				if result.err != nil && runtimeErrorCode(result.err) == "runtime_cleanup_failed" {
					_ = emit(result.report, result.err)
					return result.report, errors.Join(ctx.Err(), result.err)
				}
			}
			if r.Command == "" {
				r = report.New("dev", a.Revision)
			}
			code, action := failure(ctx.Err(), "read")
			r.AddError(code, action)
			return r, ctx.Err()
		case now := <-ticker.C:
			next, readErr := a.Projects.Read(ctx, req.root)
			if readErr != nil || next.Input.Identity.TreeDigest == "" {
				pending = devPending{}
				if result, stopped := stopCycle(); stopped {
					if result.err != nil && runtimeErrorCode(result.err) == "runtime_cleanup_failed" {
						_ = emit(result.report, result.err)
						return result.report, result.err
					}
				}
				failed, failedErr := readFailure(next, readErr)
				code := runtimeErrorCode(failedErr)
				if code == "runtime_failed" && failed.Error != nil {
					code = failed.Error.Code
				}
				failedPending.observe(next, readErr, code, now)
				if !failedPending.ready(now) {
					continue
				}
				next, readErr, code = failedPending.take()
				if code != failureCode {
					failed, failedErr = readFailure(next, readErr)
					failureCode = code
					if outputErr := emit(failed, failedErr); outputErr != nil {
						return r, outputErr
					}
				}
				continue
			}
			failedPending = devFailurePending{}
			failureCode = ""
			if last == "" {
				last = next.Input.Identity.TreeDigest
				start(next)
				continue
			}
			if next.Input.Identity.TreeDigest == last {
				if !pending.ready(now) {
					continue
				}
			} else {
				last = next.Input.Identity.TreeDigest
				pending.observe(next, now)
			}
			if !pending.ready(now) {
				continue
			}
			selected := pending.take()
			if result, stopped := stopCycle(); stopped {
				if result.err != nil && runtimeErrorCode(result.err) == "runtime_cleanup_failed" {
					_ = emit(result.report, result.err)
					return result.report, result.err
				}
			}
			start(selected)
		}
	}
}
