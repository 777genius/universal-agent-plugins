package agentpluginscli

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli/management"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/environment"
)

// NormalizeManagementStartup captures immutable trusted-host facts. A display ID
// supplied by a host never lets different roots share accepted request identity.
func NormalizeManagementStartup(input management.Startup) (management.Startup, error) {
	if input.Scope != management.User {
		return input, errors.New("this engine supports user scope only")
	}
	if input.ProjectRoot != "" {
		return input, errors.New("projectRoot requires a qualified project engine")
	}
	if len(input.Targets) == 0 || len(input.Targets) > 32 {
		return input, errors.New("one to 32 explicit targets required")
	}
	var err error
	input.StateRoot, err = managementAbsolute(input.StateRoot)
	if err != nil {
		return input, err
	}
	input.DependenciesRoot, err = managementAbsolute(input.DependenciesRoot)
	if err != nil {
		return input, err
	}
	input.Targets = append([]management.StartupTarget(nil), input.Targets...)
	ids, clients := map[string]bool{}, map[string]bool{}
	for i := range input.Targets {
		target := &input.Targets[i]
		if target.Id == "" || len(target.Id) > 128 || ids[target.Id] || clients[target.Client] {
			return input, errors.New("unique target IDs and one profile per client required")
		}
		ids[target.Id], clients[target.Client] = true, true
		target.ConfigRoot, err = managementAbsolute(target.ConfigRoot)
		if err != nil {
			return input, err
		}
		if target.Executable != "" {
			target.Executable, err = managementAbsolute(target.Executable)
			if err != nil {
				return input, err
			}
		}
	}
	if err := normalizeManagementAuthoringRoots(&input); err != nil {
		return input, err
	}
	sort.Slice(input.Targets, func(i, j int) bool { return input.Targets[i].Id < input.Targets[j].Id })
	input.ContextId = ""
	body, err := json.Marshal(input)
	if err != nil {
		return input, err
	}
	digest := sha256.Sum256(body)
	input.ContextId = hex.EncodeToString(digest[:])
	return input, nil
}

func managementAbsolute(value string) (string, error) {
	if value == "" || value != strings.TrimSpace(value) || strings.ContainsAny(value, "\x00\r\n") || !filepath.IsAbs(value) || filepath.Clean(value) != value {
		return "", errors.New("explicit absolute clean paths required")
	}
	// Resolve existing parents too, so a missing leaf below a symlink has the same
	// physical identity as its canonical spelling. No directories are created.
	current, suffix := value, []string{}
	for {
		physical, err := filepath.EvalSymlinks(current)
		if err == nil {
			for i := len(suffix) - 1; i >= 0; i-- {
				physical = filepath.Join(physical, suffix[i])
			}
			return physical, nil
		}
		if !os.IsNotExist(err) {
			return "", err
		}
		parent := filepath.Dir(current)
		if parent == current {
			return "", err
		}
		suffix = append(suffix, filepath.Base(current))
		current = parent
	}
}

type managementDetector struct {
	targets            []management.StartupTarget
	requiresExecutable func(string) bool
}

func (d managementDetector) Detect(ctx context.Context) ([]domain.DetectedClient, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	detected := make([]domain.DetectedClient, 0, len(d.targets))
	for _, target := range d.targets {
		item := domain.DetectedClient{ClientID: domain.ClientID(target.Client), ConfigRoot: target.ConfigRoot, ExecutablePath: target.Executable, Status: domain.DetectionDetected}
		if reason := managementExecutableReason(target, d.requiresExecutable(target.Client)); reason != "" {
			item.DetectionError = errors.New(reason)
		}
		detected = append(detected, item)
	}
	return detected, nil
}

// ConfigureManagementApp is called after the executable composition root creates
// the engine for StateRoot. Native profile detection is explicit in this mode.
func ConfigureManagementApp(app App, cfg management.Startup) (App, error) {
	if app.Installer == nil || app.StateStore == nil {
		return app, errors.New("installer is unavailable")
	}
	engineStateRoot, err := managementAbsolute(filepath.Dir(app.ManagedRoot))
	if err != nil || engineStateRoot != cfg.StateRoot {
		return app, errors.New("engine state root differs from captured context")
	}
	for _, target := range cfg.Targets {
		if !supportsDependencyTarget(app, target.Client) {
			return app, fmt.Errorf("unsupported dependency target %s", target.Client)
		}
	}
	app.DependenciesRoot = cfg.DependenciesRoot
	app.Detector = managementDetector{targets: append([]management.StartupTarget(nil), cfg.Targets...), requiresExecutable: app.Installer.RequiresClientExecutable}
	app.Terminal = false
	return app, nil
}

// ManagementEnvironmentID identifies the engine's durable receipt environment.
// cfg must have passed NormalizeManagementStartup. A relocated manifest or client
// executable must not hide interrupted work for the same native state store.
func ManagementEnvironmentID(cfg management.Startup) string {
	digest := sha256.Sum256([]byte(cfg.StateRoot + "\x00" + environment.EnvironmentUserDefault + "\x00" + string(cfg.Scope)))
	return hex.EncodeToString(digest[:])
}
