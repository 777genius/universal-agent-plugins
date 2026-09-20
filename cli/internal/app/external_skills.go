package app

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
)

const DefaultExternalSkillsCLIVersion = "1.5.5"

type ExternalSkillsRunner interface {
	Run(ctx context.Context, invocation ExternalSkillsInvocation) error
}

type ExternalSkillsInvocation struct {
	Command string
	Args    []string
	Stdout  io.Writer
	Stderr  io.Writer
}

type externalSkillsNpxRunner struct{}

func (externalSkillsNpxRunner) Run(ctx context.Context, invocation ExternalSkillsInvocation) error {
	cmd := exec.CommandContext(ctx, invocation.Command, invocation.Args...)
	if invocation.Stdout != nil {
		cmd.Stdout = invocation.Stdout
	} else {
		cmd.Stdout = os.Stdout
	}
	if invocation.Stderr != nil {
		cmd.Stderr = invocation.Stderr
	} else {
		cmd.Stderr = os.Stderr
	}
	return cmd.Run()
}

type ExternalSkillsInstallOptions struct {
	Source    string
	Version   string
	Global    bool
	Agents    []string
	Skills    []string
	List      bool
	Yes       bool
	Copy      bool
	All       bool
	FullDepth bool
	Stdout    io.Writer
	Stderr    io.Writer
}

type ExternalSkillsListOptions struct {
	Version string
	Global  bool
	Agents  []string
	JSON    bool
	Stdout  io.Writer
	Stderr  io.Writer
}

type ExternalSkillsUpdateOptions struct {
	Version string
	Skills  []string
	Global  bool
	Project bool
	Yes     bool
	Stdout  io.Writer
	Stderr  io.Writer
}

type ExternalSkillsRemoveOptions struct {
	Version string
	Skills  []string
	Global  bool
	Agents  []string
	Filter  []string
	Yes     bool
	All     bool
	Stdout  io.Writer
	Stderr  io.Writer
}

func (s SkillsService) InstallExternal(ctx context.Context, opts ExternalSkillsInstallOptions) error {
	source := strings.TrimSpace(opts.Source)
	if source == "" {
		return fmt.Errorf("external skills install requires source")
	}
	flags := externalSkillsCommonFlags(opts.Global, opts.Agents, opts.Skills)
	flags = appendBoolFlag(flags, "--list", opts.List)
	flags = appendBoolFlag(flags, "--yes", opts.Yes)
	flags = appendBoolFlag(flags, "--copy", opts.Copy)
	flags = appendBoolFlag(flags, "--all", opts.All)
	flags = appendBoolFlag(flags, "--full-depth", opts.FullDepth)
	invocation := externalSkillsInvocation(opts.Version, "add", []string{source}, flags, opts.Stdout, opts.Stderr)
	return s.externalSkillsRunner().Run(ctx, invocation)
}

func (s SkillsService) ListExternal(ctx context.Context, opts ExternalSkillsListOptions) error {
	flags := externalSkillsAgentFlags(nil, opts.Agents)
	flags = appendBoolFlag(flags, "--global", opts.Global)
	flags = appendBoolFlag(flags, "--json", opts.JSON)
	invocation := externalSkillsInvocation(opts.Version, "list", nil, flags, opts.Stdout, opts.Stderr)
	return s.externalSkillsRunner().Run(ctx, invocation)
}

func (s SkillsService) UpdateExternal(ctx context.Context, opts ExternalSkillsUpdateOptions) error {
	flags := make([]string, 0, 3)
	flags = appendBoolFlag(flags, "--global", opts.Global)
	flags = appendBoolFlag(flags, "--project", opts.Project)
	flags = appendBoolFlag(flags, "--yes", opts.Yes)
	invocation := externalSkillsInvocation(opts.Version, "update", cleanStrings(opts.Skills), flags, opts.Stdout, opts.Stderr)
	return s.externalSkillsRunner().Run(ctx, invocation)
}

func (s SkillsService) RemoveExternal(ctx context.Context, opts ExternalSkillsRemoveOptions) error {
	flags := externalSkillsAgentFlags(nil, opts.Agents)
	flags = appendStringFlags(flags, "--skill", opts.Filter)
	flags = appendBoolFlag(flags, "--global", opts.Global)
	flags = appendBoolFlag(flags, "--yes", opts.Yes)
	flags = appendBoolFlag(flags, "--all", opts.All)
	invocation := externalSkillsInvocation(opts.Version, "remove", cleanStrings(opts.Skills), flags, opts.Stdout, opts.Stderr)
	return s.externalSkillsRunner().Run(ctx, invocation)
}

func (s SkillsService) externalSkillsRunner() ExternalSkillsRunner {
	if s.ExternalRunner != nil {
		return s.ExternalRunner
	}
	return externalSkillsNpxRunner{}
}

func externalSkillsInvocation(version, subcommand string, positional, flags []string, stdout, stderr io.Writer) ExternalSkillsInvocation {
	args := []string{"-y", externalSkillsPackage(version), subcommand}
	args = append(args, cleanStrings(positional)...)
	args = append(args, flags...)
	return ExternalSkillsInvocation{
		Command: "npx",
		Args:    args,
		Stdout:  stdout,
		Stderr:  stderr,
	}
}

func externalSkillsPackage(version string) string {
	clean := strings.TrimSpace(version)
	if clean == "" {
		clean = DefaultExternalSkillsCLIVersion
	}
	if strings.HasPrefix(clean, "skills@") {
		return clean
	}
	return "skills@" + clean
}

func externalSkillsCommonFlags(global bool, agents, skills []string) []string {
	flags := make([]string, 0, 2+len(agents)*2+len(skills)*2)
	flags = appendBoolFlag(flags, "--global", global)
	flags = externalSkillsAgentFlags(flags, agents)
	flags = appendStringFlags(flags, "--skill", skills)
	return flags
}

func externalSkillsAgentFlags(flags []string, agents []string) []string {
	return appendStringFlags(flags, "--agent", agents)
}

func appendStringFlags(flags []string, name string, values []string) []string {
	for _, value := range cleanStrings(values) {
		flags = append(flags, name, value)
	}
	return flags
}

func appendBoolFlag(flags []string, name string, enabled bool) []string {
	if enabled {
		flags = append(flags, name)
	}
	return flags
}

func cleanStrings(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		if clean := strings.TrimSpace(value); clean != "" {
			out = append(out, clean)
		}
	}
	return out
}
