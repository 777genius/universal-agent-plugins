//go:build linux

package main

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"time"

	"golang.org/x/sys/unix"
)

func (c *controller) finish(runErr, cleanupErr error, out, stderr []byte) error {
	if e := c.keep("root-cleanup.stdout.log", out); e != nil {
		cleanupErr = errors.Join(cleanupErr, e)
	}
	if e := c.keep("root-cleanup.stderr.log", stderr); e != nil {
		cleanupErr = errors.Join(cleanupErr, e)
	}
	outHash, errHash := sha256.Sum256(out), sha256.Sum256(stderr)
	summary := map[string]any{"evidence_kind": "vscode-pinned-untrusted-workspace-research", "source_commit": buildCommit, "version": publisherVersion, "publisher_commit": publisherCommit, "collection_complete": runErr == nil && cleanupErr == nil, "native_project_qualified": false, "native_mcp_initialization_qualified": false, "native_runtime_qualified": false, "children": c.children, "observations": c.observations, "executable_hashes": c.hashes, "error": clipped(runErr), "cleanup": map[string]any{"complete": cleanupErr == nil, "error": clipped(cleanupErr), "stdout_file": "root-cleanup.stdout.log", "stderr_file": "root-cleanup.stderr.log", "stdout_bytes": len(out), "stderr_bytes": len(stderr), "stdout_sha256": hex.EncodeToString(outHash[:]), "stderr_sha256": hex.EncodeToString(errHash[:])}, "diagnostics": "verifier-prefixed merged streams; planned teardown may truncate trailing diagnostics; API result is separate"}
	if c.request.Input.Mode == trustedProjectMode {
		c.trustedSummary(summary, runErr, cleanupErr)
	}
	body, e := encode(summary, 1<<20)
	if e != nil {
		return errors.Join(runErr, cleanupErr, e)
	}
	if e = c.keep("vscode-workspace-controller.json", body); e != nil {
		return errors.Join(runErr, cleanupErr, e)
	}
	if e = c.publish(); e != nil {
		return errors.Join(runErr, cleanupErr, e)
	}
	public, e := encode(map[string]any{"collection_complete": runErr == nil && cleanupErr == nil, "cleanup_complete": cleanupErr == nil, "error": clipped(errors.Join(runErr, cleanupErr))}, controlLimit)
	if e != nil {
		return errors.Join(runErr, cleanupErr, e)
	}
	_, e = os.Stdout.Write(public)
	return errors.Join(runErr, cleanupErr, e)
}

func (c *controller) publish() error {
	before, err := c.output.Stat()
	if err != nil {
		return err
	}
	current, err := os.Lstat(c.request.Input.OutputRoot)
	if err != nil || !os.SameFile(before, current) || !current.IsDir() {
		return fmt.Errorf("evidence output root changed")
	}
	for _, name := range sortedKeys(c.files) {
		deadline, e := inputDeadline(c.request.Input, "publish")
		if e != nil || time.Until(deadline) <= 0 {
			return fmt.Errorf("controller publication deadline")
		}
		fd, err := unix.Openat(int(c.output.Fd()), name, unix.O_WRONLY|unix.O_CREAT|unix.O_EXCL|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0600)
		if err != nil {
			return err
		}
		f := os.NewFile(uintptr(fd), name)
		_, err = f.Write(c.files[name])
		if err == nil {
			err = f.Chown(c.request.Input.UID, c.request.Input.GID)
		}
		closeErr := f.Close()
		if err != nil || closeErr != nil {
			return errors.Join(err, closeErr)
		}
	}
	return nil
}
