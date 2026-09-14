package commands

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
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

func acquireDevSession(ctx context.Context, scratch, root string) (func() error, error) {
	canonical, err := filepath.Abs(root)
	if err != nil {
		return nil, &mcpruntime.Error{Code: "runtime_dev_session_lock_unavailable"}
	}
	canonical, err = filepath.EvalSymlinks(canonical)
	if err != nil {
		return nil, &mcpruntime.Error{Code: "runtime_dev_session_lock_unavailable"}
	}
	digest := sha256.Sum256([]byte(filepath.Clean(canonical)))
	lockPath := filepath.Join(scratch, "agentplugins-author-dev-locks", hex.EncodeToString(digest[:])+".lock")
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
	p, err := a.Projects.Read(ctx, req.root)
	r = report.Build("dev", a.Revision, p, req.release)
	if err != nil {
		code, action := failure(err, "read")
		r.AddError(code, action)
		return r, err
	}
	last := p.Input.Identity.TreeDigest
	if last == "" {
		r.AddError("runtime_source_identity_unavailable", "Make the exact package tree readable and quiescent, then retry.")
		return r, errors.New("runtime source identity unavailable")
	}
	release, lockErr := acquireDevSession(ctx, a.Projects.Scratch, req.root)
	if lockErr != nil {
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
	var cycle context.CancelFunc
	var done chan cycleResult
	start := func(p project.Result) {
		cycleCtx, cancel := context.WithCancel(ctx)
		cycle = cancel
		done = make(chan cycleResult, 1)
		go func() {
			result := report.Build("dev", a.Revision, p, req.release)
			evidence, runErr := mcpruntime.Run(cycleCtx, mcpruntime.Options{SourceRoot: req.root, Scratch: a.Projects.Scratch, Project: p, Server: req.server, Tool: req.tool, Fixture: req.fixture, AllowNetwork: req.allowNetwork, Deadline: req.deadline, Projects: a.Projects})
			result.SetRuntime(evidence.Transport, evidence.Initialize, evidence.ListTools, evidence.ToolCall, evidence.ToolCount, evidence.Cleanup, runtimeErrorCode(runErr))
			done <- cycleResult{result, runErr}
		}()
	}
	start(p)
	ticker := time.NewTicker(devPollInterval)
	defer ticker.Stop()
	var pending devPending
	for {
		select {
		case result := <-done:
			r = result.report
			done = nil
			cycle = nil
			if result.err != nil {
				return r, result.err
			}
		case <-ctx.Done():
			if cycle != nil {
				cycle()
				result := <-done
				if result.err != nil && runtimeErrorCode(result.err) == "runtime_cleanup_failed" {
					return result.report, errors.Join(ctx.Err(), result.err)
				}
			}
			return r, ctx.Err()
		case now := <-ticker.C:
			next, readErr := a.Projects.Read(ctx, req.root)
			if readErr != nil {
				if cycle != nil {
					cycle()
					result := <-done
					if result.err != nil && runtimeErrorCode(result.err) == "runtime_cleanup_failed" {
						return result.report, errors.Join(readErr, result.err)
					}
				}
				r = report.Build("dev", a.Revision, next, req.release)
				code, action := failure(readErr, "read")
				r.AddError(code, action)
				return r, readErr
			}
			if next.Input.Identity.TreeDigest == "" {
				identityErr := errors.New("runtime source identity unavailable")
				if cycle != nil {
					cycle()
					result := <-done
					if result.err != nil && runtimeErrorCode(result.err) == "runtime_cleanup_failed" {
						return result.report, errors.Join(identityErr, result.err)
					}
				}
				r = report.Build("dev", a.Revision, next, req.release)
				r.AddError("runtime_source_identity_unavailable", "Make the exact package tree readable and quiescent, then retry.")
				return r, identityErr
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
			if cycle != nil {
				cycle()
				result := <-done
				cycle = nil
				done = nil
				if result.err != nil && runtimeErrorCode(result.err) == "runtime_cleanup_failed" {
					return result.report, result.err
				}
			}
			start(selected)
		}
	}
}
