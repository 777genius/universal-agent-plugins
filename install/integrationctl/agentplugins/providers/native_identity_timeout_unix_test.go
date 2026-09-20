//go:build !windows

package providers

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	processadapter "github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/process"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func TestNativeIdentityTimeoutReapsAuthoritativeDiscoveryChild(t *testing.T) {
	requireNativeIdentityTreeCapability(t)
	root := t.TempDir()
	pidPath := filepath.Join(root, "child.pid")
	executable := filepath.Join(root, "copilot")
	script := "#!/bin/sh\nsleep 60 &\nprintf '%s %s' \"$$\" \"$!\" > \"$AGENTPLUGINS_TEST_PID\"\nwait\n"
	if err := os.WriteFile(executable, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("AGENTPLUGINS_TEST_PID", pidPath)
	plan := identityPlan(filepath.Join(root, "prepared"))
	plan.NativeRegistryExecutable = executable
	observation, err := (testObserver(NativeIdentityObserver{
		// Give the disposable shell enough time to create its PID handshake even
		// when the full package suite is saturating the host. The observed failure
		// is still the runner deadline, not a test-side kill.
		Runner: processadapter.OS{}, DiscoveryTimeout: 3 * time.Second,
	})).ObserveNativeIdentity(context.Background(), domain.DetectedClient{ClientID: domain.ClientCopilot}, plan, nil)
	if !errors.Is(err, context.DeadlineExceeded) || observation.State != domain.NativeIdentityIndeterminate {
		t.Fatalf("observation = %+v, err = %v", observation, err)
	}
	body, readErr := os.ReadFile(pidPath)
	if readErr != nil {
		t.Fatalf("read child pid: %v", readErr)
	}
	pidFields := strings.Fields(string(body))
	if len(pidFields) != 2 {
		t.Fatalf("process-group pids = %q", body)
	}
	for _, field := range pidFields {
		pid, parseErr := strconv.Atoi(field)
		if parseErr != nil {
			t.Fatalf("parse process-group pid %q: %v", field, parseErr)
		}
		if err := waitProcessGone(pid, 2*time.Second); err != nil {
			_ = syscall.Kill(pid, syscall.SIGKILL)
			t.Fatal(err)
		}
	}
}

func TestNativeIdentityNormalExitCleansSameGroupMemberBeforeReapingLeader(t *testing.T) {
	requireNativeIdentityTreeCapability(t)
	root := t.TempDir()
	pidPath := filepath.Join(root, "grandchild.pid")
	executable := filepath.Join(root, "copilot")
	script := "#!/bin/sh\nsleep 60 &\nprintf '%s' \"$!\" > \"$AGENTPLUGINS_TEST_PID\"\nexit 0\n"
	if err := os.WriteFile(executable, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("AGENTPLUGINS_TEST_PID", pidPath)
	plan := identityPlan(filepath.Join(root, "prepared"))
	plan.NativeRegistryExecutable = executable
	started := time.Now()
	observation, err := (testObserver(NativeIdentityObserver{
		Runner: processadapter.OS{}, DiscoveryTimeout: 5 * time.Second,
	})).ObserveNativeIdentity(context.Background(), domain.DetectedClient{ClientID: domain.ClientCopilot}, plan, nil)
	if err == nil || !strings.Contains(err.Error(), "live descendants that required forced cleanup") || observation.State != domain.NativeIdentityIndeterminate {
		t.Fatalf("observation = %+v, err = %v, want forced-descendant uncertainty", observation, err)
	}
	if elapsed := time.Since(started); elapsed > 2*time.Second {
		t.Fatalf("normal-exit background cleanup took %s", elapsed)
	}
	body, readErr := os.ReadFile(pidPath)
	if readErr != nil {
		t.Fatalf("read grandchild pid: %v", readErr)
	}
	pid, parseErr := strconv.Atoi(strings.TrimSpace(string(body)))
	if parseErr != nil {
		t.Fatalf("parse grandchild pid: %v", parseErr)
	}
	if err := waitProcessGone(pid, 2*time.Second); err != nil {
		_ = syscall.Kill(pid, syscall.SIGKILL)
		t.Fatalf("normal-exit process-group member was not cleaned before leader reap: %v", err)
	}
}

func requireNativeIdentityTreeCapability(t *testing.T) {
	t.Helper()
	if runtime.GOOS != "linux" {
		return
	}
	if err := (processadapter.OS{}).DuplexCapability(); err != nil {
		t.Skipf("atomic Linux process-tree containment unavailable: %v", err)
	}
}

func waitProcessGone(pid int, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for {
		err := syscall.Kill(pid, 0)
		if errors.Is(err, syscall.ESRCH) {
			return nil
		}
		if runtime.GOOS == "linux" {
			if stat, readErr := os.ReadFile(filepath.Join("/proc", strconv.Itoa(pid), "stat")); readErr == nil {
				fields := strings.Fields(string(stat))
				if len(fields) > 2 && fields[2] == "Z" {
					return nil
				}
			}
		}
		if err != nil {
			return fmt.Errorf("inspect process %d: %w", pid, err)
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("authoritative discovery process %d remained alive after timeout", pid)
		}
		time.Sleep(20 * time.Millisecond)
	}
}
