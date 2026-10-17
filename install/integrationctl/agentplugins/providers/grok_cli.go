package providers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	legacyports "github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

type grokStatus int

const (
	grokStatusUnknown grokStatus = iota
	grokStatusInstalled
	grokStatusAbsent
)

var errGrokListContractUnknown = errors.New("Grok plugin list output is not recognized")

func (activator Activator) activateGrok(ctx context.Context, request domain.ActivationRequest) error {
	if request.Replacing {
		_, _ = activator.runGrokResult(ctx, request.BackendExecutable, "plugin", "uninstall", "--confirm", request.DeclaredName)
	}
	_, installErr := activator.runGrokResult(ctx, request.BackendExecutable, "plugin", "install", "--trust", request.Delivery.ActivePath)
	if err := activator.verifyGrok(ctx, request); err == nil {
		return nil
	} else if installErr != nil {
		return fmt.Errorf("activate Grok plugin: %w", installErr)
	} else {
		return err
	}
}

func (activator Activator) verifyGrok(ctx context.Context, request domain.ActivationRequest) error {
	listed, err := activator.runGrokResult(ctx, request.BackendExecutable, "plugin", "list", "--json")
	if err != nil {
		return fmt.Errorf("verify Grok plugin listing: %w", err)
	}
	switch grokPluginStatus(listed.Stdout, request.DeclaredName) {
	case grokStatusInstalled:
		return nil
	case grokStatusAbsent:
		return fmt.Errorf("%w: verify Grok plugin listing: %s is not installed", errRecognizedNegativeEvidence, request.DeclaredName)
	default:
		return fmt.Errorf("%w: verify Grok plugin listing", errGrokListContractUnknown)
	}
}

func (activator Activator) deactivateGrok(ctx context.Context, request domain.DeactivationRequest) error {
	_, err := activator.runGrokResult(ctx, request.BackendExecutable, "plugin", "uninstall", "--confirm", request.DeclaredName)
	if err == nil {
		return nil
	}
	listed, listErr := activator.runGrokResult(ctx, request.BackendExecutable, "plugin", "list", "--json")
	if listErr != nil {
		return fmt.Errorf("uninstall Grok plugin: %v; verify listing: %w", err, listErr)
	}
	switch grokPluginStatus(listed.Stdout, request.DeclaredName) {
	case grokStatusAbsent:
		return nil
	case grokStatusInstalled:
		return fmt.Errorf("uninstall Grok plugin: %w", err)
	default:
		return fmt.Errorf("%w: verify Grok plugin listing after uninstall", errGrokListContractUnknown)
	}
}

func (activator Activator) runGrokResult(ctx context.Context, executable string, args ...string) (legacyports.CommandResult, error) {
	return activator.runClientResult(ctx, "Grok CLI", executable, args...)
}

func grokPluginStatus(body []byte, name string) grokStatus {
	name = strings.TrimSpace(name)
	if name == "" || len(body) == 0 {
		return grokStatusUnknown
	}
	decoder := json.NewDecoder(strings.NewReader(string(body)))
	decoder.UseNumber()
	parsed, err := decodeUniqueJSONValue(decoder)
	if err != nil {
		return grokStatusUnknown
	}
	if _, tokenErr := decoder.Token(); tokenErr == nil || !errors.Is(tokenErr, io.EOF) {
		return grokStatusUnknown
	}
	entries, ok := parsed.([]any)
	if !ok {
		return grokStatusUnknown
	}
	seen := map[string]struct{}{}
	found := false
	for _, value := range entries {
		entry, ok := value.(map[string]any)
		if !ok {
			return grokStatusUnknown
		}
		entryName, nameOK := entry["name"].(string)
		status, statusOK := entry["status"].(string)
		entryName = strings.TrimSpace(entryName)
		status = strings.TrimSpace(status)
		if !nameOK || !statusOK || entryName == "" || (status != "installed" && status != "disabled") {
			return grokStatusUnknown
		}
		if _, duplicate := seen[entryName]; duplicate {
			return grokStatusUnknown
		}
		seen[entryName] = struct{}{}
		if entryName == name {
			found = true
		}
	}
	if found {
		return grokStatusInstalled
	}
	return grokStatusAbsent
}

func grokRegistryFinding(body []byte, name string, owned bool) registryFinding {
	switch grokPluginStatus(body, name) {
	case grokStatusInstalled:
		if owned {
			return registryExpected
		}
		return registryCollision
	case grokStatusAbsent:
		return registryClear
	default:
		return registryIndeterminate
	}
}
