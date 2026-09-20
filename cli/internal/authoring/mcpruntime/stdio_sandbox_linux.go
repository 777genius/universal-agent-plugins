//go:build linux

package mcpruntime

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const (
	containedPackageRoot = "/plugin"
	containedDataRoot    = "/data"
)

// stdioSandboxCommand admits package execution only through bubblewrap's mount,
// user, PID, and network namespaces. The child sees the private package copy,
// its writable operation data, and read-only system runtime directories; it
// does not see the source project, host home/configuration, or host network.
func stdioSandboxCommand(ctx context.Context, root, data, node string, args []string, cwd string) ([]string, string, string, string, error) {
	bwrap, err := systemExecutable("bwrap", "/usr/bin/bwrap", "/bin/bwrap")
	if err != nil {
		return nil, "", "", "", err
	}
	node, err = filepath.EvalSymlinks(node)
	if err != nil || !withinSystemRuntime(node) {
		return nil, "", "", "", fmt.Errorf("node is outside the contained system runtime")
	}
	profile := []string{
		"--unshare-all", "--die-with-parent", "--new-session", "--cap-drop", "ALL",
		"--tmpfs", "/", "--ro-bind", "/usr", "/usr", "--ro-bind-try", "/bin", "/bin",
		"--ro-bind-try", "/lib", "/lib", "--ro-bind-try", "/lib64", "/lib64",
		"--dev", "/dev", "--proc", "/proc", "--dir", "/tmp",
		"--ro-bind", root, containedPackageRoot, "--bind", data, containedDataRoot,
	}
	sandboxCWD, err := sandboxPath(cwd, root, data)
	if err != nil {
		return nil, "", "", "", err
	}
	profile = append(profile, "--chdir", sandboxCWD)

	// A successful, bounded probe proves this exact host can establish the
	// namespace profile before any package-authored process is attempted.
	probeCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	probe := exec.CommandContext(probeCtx, bwrap, append(append([]string{}, profile...), "--", node, "-e", "process.exit(0)")...)
	probe.Env = []string{"HOME=" + containedDataRoot + "/home", "TMPDIR=" + containedDataRoot + "/tmp", "PATH=" + filepath.Dir(node)}
	if err := probe.Run(); err != nil {
		return nil, "", "", "", fmt.Errorf("bubblewrap containment probe failed: %w", err)
	}

	replacer := strings.NewReplacer(root, containedPackageRoot, data, containedDataRoot)
	sandboxArgs := make([]string, len(args))
	for i, arg := range args {
		sandboxArgs[i] = replacer.Replace(arg)
	}
	command := append(append(append([]string{bwrap}, profile...), "--", node), sandboxArgs...)
	return command, containedPackageRoot, containedDataRoot, node, nil
}

func systemExecutable(name string, candidates ...string) (string, error) {
	for _, candidate := range candidates {
		info, err := os.Lstat(candidate)
		if err == nil && info.Mode().IsRegular() && info.Mode()&0111 != 0 {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("system %s is unavailable", name)
}

func withinSystemRuntime(name string) bool {
	for _, base := range []string{"/usr", "/bin", "/lib", "/lib64"} {
		if rel, err := filepath.Rel(base, name); err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return true
		}
	}
	return false
}

func sandboxPath(name, root, data string) (string, error) {
	for _, item := range []struct{ host, sandbox string }{{root, containedPackageRoot}, {data, containedDataRoot}} {
		rel, err := filepath.Rel(item.host, name)
		if err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			if rel == "." {
				return item.sandbox, nil
			}
			return filepath.ToSlash(filepath.Join(item.sandbox, rel)), nil
		}
	}
	return "", fmt.Errorf("sandbox working directory is outside private roots")
}
