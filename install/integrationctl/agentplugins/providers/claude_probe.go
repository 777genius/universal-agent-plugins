package providers

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	legacyports "github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

const (
	claudeProbeMaxEnvironmentValue = 4096
	claudeProbeMaxEnvironmentTotal = 32768
	claudeProbeTreeExitGrace       = 5 * time.Second
	claudeProbeTimeout             = 15 * time.Second
)

type claudeDescendantGraceRunner interface {
	RunWithDescendantExitGrace(context.Context, legacyports.Command, time.Duration) (legacyports.CommandResult, error)
}

type claudeActivationProbe struct {
	command    legacyports.Command
	configRoot string
	activePath string
}

func prepareClaudeActivationProbe(request domain.ActivationRequest) (claudeActivationProbe, error) {
	configRoot := filepath.Clean(strings.TrimSpace(request.Client.ConfigRoot))
	if !filepath.IsAbs(configRoot) {
		return claudeActivationProbe{}, fmt.Errorf("absolute Claude Code config root is required")
	}
	targetAnchor := filepath.Clean(strings.TrimSpace(request.Plan.TargetAnchor))
	if !filepath.IsAbs(targetAnchor) || targetAnchor != configRoot {
		return claudeActivationProbe{}, fmt.Errorf("Claude Code delivery anchor must match the configured root")
	}
	targetRoot := filepath.Clean(strings.TrimSpace(request.Plan.TargetRoot))
	if targetRoot != filepath.Join(configRoot, "skills") {
		return claudeActivationProbe{}, fmt.Errorf("Claude Code delivery root must be the exact configured skills root")
	}
	activePath := filepath.Clean(strings.TrimSpace(request.Plan.ActivePath))
	if !filepath.IsAbs(activePath) || filepath.Dir(activePath) != targetRoot {
		return claudeActivationProbe{}, fmt.Errorf("Claude Code managed plugin path is not an exact child of the configured skills root")
	}
	if deliveryPath := strings.TrimSpace(request.Delivery.ActivePath); deliveryPath != "" && filepath.Clean(deliveryPath) != activePath {
		return claudeActivationProbe{}, fmt.Errorf("Claude Code activation path does not match the preflighted delivery path")
	}
	command, err := claudeListCommand(request.BackendExecutable, configRoot, activePath)
	if err != nil {
		return claudeActivationProbe{}, err
	}
	return claudeActivationProbe{command: command, configRoot: configRoot, activePath: activePath}, nil
}

func runClaudeListCommand(ctx context.Context, runner CommandRunner, command legacyports.Command) (legacyports.CommandResult, error) {
	if runner == nil {
		return legacyports.CommandResult{}, fmt.Errorf("Claude Code CLI runner is unavailable")
	}
	bounded, cancel := context.WithTimeout(ctx, claudeProbeTimeout)
	defer cancel()
	if supervised, ok := runner.(claudeDescendantGraceRunner); ok {
		return supervised.RunWithDescendantExitGrace(bounded, command, claudeProbeTreeExitGrace)
	}
	return runner.Run(bounded, command)
}

func claudeListCommand(executable, configRoot, activePath string) (legacyports.Command, error) {
	executable = strings.TrimSpace(executable)
	if executable == "" {
		return legacyports.Command{}, fmt.Errorf("Claude Code executable is required")
	}
	configRoot = strings.TrimSpace(configRoot)
	if configRoot == "" || !filepath.IsAbs(configRoot) {
		return legacyports.Command{}, fmt.Errorf("absolute Claude Code config root is required")
	}
	configRoot = filepath.Clean(configRoot)
	activePath = filepath.Clean(strings.TrimSpace(activePath))
	if !filepath.IsAbs(activePath) || filepath.Dir(activePath) != filepath.Join(configRoot, "skills") {
		return legacyports.Command{}, fmt.Errorf("Claude Code managed plugin path is not an exact child of the configured skills root")
	}
	environment, home, err := boundedClaudeProbeEnvironment()
	if err != nil {
		return legacyports.Command{}, err
	}
	environment = append(environment,
		"CLAUDE_CONFIG_DIR="+configRoot,
		"DISABLE_AUTOUPDATER=1",
		"CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC=1",
	)
	if environmentSize(environment) > claudeProbeMaxEnvironmentTotal {
		return legacyports.Command{}, fmt.Errorf("Claude Code probe environment exceeds the bounded size")
	}
	return legacyports.Command{
		Argv: []string{executable, "plugin", "list", "--json"},
		Env:  environment,
		Dir:  home,
	}, nil
}

func boundedClaudeProbeEnvironment() ([]string, string, error) {
	return boundedClaudeProbeEnvironmentFrom(os.Environ())
}

func boundedClaudeProbeEnvironmentFrom(ambient []string) ([]string, string, error) {
	values := map[string]string{}
	for _, entry := range ambient {
		name, value, ok := strings.Cut(entry, "=")
		if !ok || name == "" || !allowedClaudeProbeEnvironment(name) {
			continue
		}
		canonical := strings.ToUpper(name)
		if _, duplicate := values[canonical]; duplicate {
			return nil, "", fmt.Errorf("Claude Code probe environment contains duplicate %s", name)
		}
		if len(value) > claudeProbeMaxEnvironmentValue || strings.ContainsRune(value, '\x00') {
			return nil, "", fmt.Errorf("Claude Code probe environment value %s is invalid or too large", name)
		}
		values[canonical] = value
	}
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	environment := make([]string, 0, len(keys)+3)
	for _, key := range keys {
		environment = append(environment, key+"="+values[key])
	}
	homeKey := "HOME"
	if runtime.GOOS == "windows" {
		homeKey = "USERPROFILE"
	}
	home := strings.TrimSpace(values[homeKey])
	if home == "" || !filepath.IsAbs(home) {
		return nil, "", fmt.Errorf("Claude Code probe requires an absolute real %s", homeKey)
	}
	info, err := os.Lstat(home)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return nil, "", fmt.Errorf("Claude Code probe requires a real %s directory", homeKey)
	}
	if environmentSize(environment) > claudeProbeMaxEnvironmentTotal {
		return nil, "", fmt.Errorf("Claude Code probe environment exceeds the bounded size")
	}
	return environment, filepath.Clean(home), nil
}

func allowedClaudeProbeEnvironment(name string) bool {
	upper := strings.ToUpper(name)
	if strings.HasPrefix(upper, "LC_") {
		return true
	}
	switch upper {
	case "HOME", "USER", "LOGNAME", "USERPROFILE", "PATH",
		"TMPDIR", "TMP", "TEMP", "LANG",
		"SYSTEMROOT", "WINDIR", "COMSPEC", "PATHEXT", "APPDATA", "LOCALAPPDATA":
		return true
	default:
		return false
	}
}

func environmentSize(environment []string) int {
	total := 0
	for _, entry := range environment {
		total += len(entry) + 1
	}
	return total
}
