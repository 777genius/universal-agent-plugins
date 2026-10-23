package sourceacquisition

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"time"

	processadapter "github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/process"
	legacyports "github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

// Native authentication belongs to the user, not the fetched repository. Keep
// HOME/config/agent and explicit authentication inputs while stripping routing,
// object injection, traces and arbitrary runtime Git configuration.
func nativeGitEnvironment(environ []string) []string {
	clean := make([]string, 0, len(environ)+4)
	for _, item := range environ {
		key, _, _ := strings.Cut(item, "=")
		upper := strings.ToUpper(key)
		if strings.HasPrefix(upper, "GIT_") {
			switch upper {
			case "GIT_ASKPASS", "GIT_SSH", "GIT_SSH_COMMAND", "GIT_SSH_VARIANT", "GIT_SSL_CAINFO", "GIT_SSL_CAPATH":
			default:
				continue
			}
		}
		if upper == "GCM_INTERACTIVE" || upper == "SSH_ASKPASS_REQUIRE" {
			continue
		}
		clean = append(clean, item)
	}
	return append(clean, "GIT_TERMINAL_PROMPT=0", "GIT_LFS_SKIP_SMUDGE=1", "GCM_INTERACTIVE=Never", "SSH_ASKPASS_REQUIRE=never", "GIT_ALLOW_PROTOCOL=https:ssh")
}

func nativeGitOptions(workspace string) []string {
	return []string{
		"-c", "core.hooksPath=" + filepath.Join(workspace, "disabled-hooks"),
		"-c", "core.fsmonitor=false", "-c", "maintenance.auto=false", "-c", "gc.auto=0",
		"-c", "fetch.recurseSubmodules=false", "-c", "submodule.recurse=false",
		"-c", "http.sslVerify=true", "-c", "protocol.allow=never",
		"-c", "protocol.https.allow=always", "-c", "protocol.ssh.allow=always",
	}
}

// Git's core.sshCommand is trusted user configuration. Read it without exposing
// its text, then append only a fixed batch option. No source text enters a shell
// command. Native Git still supplies/quotes the host and repository arguments.
func withBatchSSH(ctx context.Context, environ []string, dir string) []string {
	command, variant, wrapper := "", "", false
	for _, item := range environ {
		key, value, _ := strings.Cut(item, "=")
		switch key {
		case "GIT_SSH_COMMAND":
			command = value
		case "GIT_SSH_VARIANT":
			variant = value
		case "GIT_SSH":
			wrapper = value != ""
		}
	}
	if wrapper && command == "" {
		return environ
	} // an explicit trusted wrapper owns its arguments
	if command == "" {
		result, err := (processadapter.OS{}).RunWithTreeExitGrace(ctx, legacyports.Command{Argv: []string{"git", "config", "--get", "core.sshCommand"}, Dir: dir, Env: environ, StdoutLimitBytes: 8192}, time.Second)
		if err == nil && result.ExitCode == 0 {
			command = strings.TrimSpace(string(result.Stdout))
		}
	}
	if command == "" {
		command = "ssh"
	}
	if variant == "plink" || variant == "tortoiseplink" || variant == "putty" {
		command += " -batch"
	} else {
		command += " -oBatchMode=yes"
	}
	clean := make([]string, 0, len(environ)+1)
	for _, item := range environ {
		if !strings.HasPrefix(item, "GIT_SSH_COMMAND=") {
			clean = append(clean, item)
		}
	}
	return append(clean, "GIT_SSH_COMMAND="+command)
}

func ctxOrError(ctx context.Context, err error) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	return err
}

// URL-specific Git configuration outranks http.sslVerify. Override every
// configured spelling at its own specificity, using the same private repository
// context as the fetch. Values (which may contain credentials) are never read.
// Trusted user configuration is assumed stable during this native command.
func nativeVerificationOptions(ctx context.Context, environ []string, command Command) ([]string, error) {
	options := nativeGitOptions(command.Dir)
	args := []string{"git"}
	if len(command.Args) >= 2 && command.Args[0] == "-C" {
		args = append(args, command.Args[:2]...)
	}
	args = append(args, "config", "--null", "--name-only", "--get-regexp", `^http(\..*)?\.sslverify$`)
	result, err := (processadapter.OS{}).RunWithTreeExitGrace(ctx, legacyports.Command{Argv: args, Dir: command.Dir, Env: environ, StdoutLimitBytes: 1 << 20}, time.Second)
	if err != nil || (result.ExitCode != 0 && result.ExitCode != 1) {
		return nil, errors.New("cannot enforce native Git TLS verification")
	}
	for _, key := range strings.Split(string(result.Stdout), "\x00") {
		if key == "" {
			continue
		}
		if len(key) > 8192 || strings.ContainsAny(key, "\r\n") {
			return nil, errors.New("native Git TLS configuration key is invalid")
		}
		options = append(options, "-c", key+"=true")
	}
	return options, nil
}
