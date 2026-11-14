package hermes

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	legacyports "github.com/777genius/plugin-kit-ai/install/integrationctl/ports"

	"gopkg.in/yaml.v3"
)

// Explicit run consent admits these three bounded native observations. They do
// not connect MCP or call a model; any native bootstrap remains in its selected
// runtime root and must pass the same qualified source boundary afterwards.
func verifySessionNativeSource(ctx context.Context, env clients.Env, plan clients.SessionPlan) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if env.Runner == nil {
		return fmt.Errorf("hermes native configuration observer unavailable")
	}
	if err := verifySessionAuthority(ctx, env, plan); err != nil {
		return err
	}
	body, err := projectSessionConfig(ctx, plan.Directory, sessionRootObject(plan.Directory), plan.Servers)
	if err != nil {
		return err
	}
	bounded, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	bundle, err := qualifyHermesBundle(plan.Executable)
	if err != nil {
		return err
	}
	version, err := observeSessionConfig(bounded, env, plan, "--version")
	if err != nil {
		return err
	}
	if err := hermesInstallDirectory(version, filepath.Join(bundle.root, "hermes-agent")); err != nil {
		return err
	}
	path, err := observeSessionConfig(bounded, env, plan, "config", "path")
	if err != nil {
		return err
	}
	expected := filepath.Join(projectSessionRoot(plan.Directory), "config.yaml")
	if string(path) != expected+"\n" && string(path) != expected+"\r\n" {
		return fmt.Errorf("hermes selected a different native configuration path")
	}
	servers, err := observeSessionConfig(bounded, env, plan, "config", "get", "mcp_servers", "--json")
	if err != nil {
		return err
	}
	if err := compareSessionNativeSource(body, servers); err != nil {
		return err
	}
	return verifySessionAuthority(bounded, env, plan)
}

func observeSessionConfig(ctx context.Context, env clients.Env, plan clients.SessionPlan, arguments ...string) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := verifySessionAuthority(ctx, env, plan); err != nil {
		return nil, err
	}
	environment := plan.ChildEnvironment(os.Environ())
	if err := hermesSessionSignals(projectSessionRoot(plan.Directory), environment); err != nil {
		return nil, err
	}
	const limit = 1 << 20
	result, err := env.Runner.Run(ctx, legacyports.Command{
		Argv: append([]string{plan.Executable}, arguments...), Dir: plan.Directory,
		Env: environment, StdoutLimitBytes: limit,
	})
	if err != nil {
		return nil, fmt.Errorf("hermes native configuration observation failed: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if result.ExitCode != 0 || len(result.Stdout) >= limit || len(result.Stderr) > 32768 || !result.CleanupComplete || result.NoChildStarted {
		return nil, fmt.Errorf("hermes configuration observation failed, exceeded its bound or lacked process cleanup")
	}
	if err := verifySessionAuthority(ctx, env, plan); err != nil {
		return nil, err
	}
	return result.Stdout, nil
}

func hermesInstallDirectory(body []byte, expected string) error {
	lines := strings.Split(strings.ReplaceAll(string(body), "\r\n", "\n"), "\n")
	if lines[0] != "Hermes Agent vunknown (2026.9.24)" {
		return fmt.Errorf("hermes native version differs from qualified bundle")
	}
	found := 0
	for _, line := range lines[1:] {
		if strings.HasPrefix(line, "Hermes Agent v") {
			return fmt.Errorf("hermes native version observation is ambiguous")
		}
		if value, ok := strings.CutPrefix(line, "Install directory: "); ok {
			found++
			if value != expected {
				return fmt.Errorf("hermes native install directory differs from selected bundle")
			}
		}
	}
	if found != 1 {
		return fmt.Errorf("hermes native version/install-directory observation is missing")
	}
	return nil
}

func verifySessionAuthority(ctx context.Context, env clients.Env, plan clients.SessionPlan) error {
	if env.ProjectRoots == nil {
		return fmt.Errorf("hermes selected-root verifier is unavailable")
	}
	owner := sessionRootObject(plan.Directory)
	if err := env.ProjectRoots.VerifyProjectRoot(ctx, projectSessionRoot(plan.Directory), owner); err != nil {
		return err
	}
	return verifyProjectSessionSource(ctx, plan, owner)
}

func compareSessionNativeSource(source, observed []byte) error {
	var config struct {
		Servers map[string]any `yaml:"mcp_servers"`
	}
	if err := yaml.Unmarshal(source, &config); err != nil {
		return fmt.Errorf("hermes owned YAML could not be compared")
	}
	wanted, err := json.Marshal(config.Servers)
	if err != nil {
		return err
	}
	document, err := shared.DecodeStrictJSONObject(observed)
	if err != nil {
		return fmt.Errorf("hermes native MCP observation is not one unambiguous JSON object")
	}
	actual, err := json.Marshal(document)
	if err != nil || !bytes.Equal(wanted, actual) {
		return fmt.Errorf("hermes effective MCP differs from exact owned Project source")
	}
	return nil
}
