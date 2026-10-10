package grok

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	legacyports "github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

const (
	grokListTimeout          = 15 * time.Second
	grokListStdoutLimitBytes = 1 << 20
)

var errUnknownList = errors.New("grok plugin list JSON contract is not recognized")

type pluginEntry struct {
	Name    string `json:"name"`
	Status  string `json:"status"`
	Source  string `json:"source"`
	Version string `json:"version"`
}

// parseList accepts only a complete, unambiguous list. A missing or changed
// native contract must never become evidence that a name is available.
func parseList(body []byte) ([]pluginEntry, error) {
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.UseNumber()
	parsed, err := shared.DecodeUniqueJSONValue(decoder)
	if err != nil {
		return nil, errUnknownList
	}
	if _, err := decoder.Token(); !errors.Is(err, io.EOF) {
		return nil, errUnknownList
	}
	values, ok := parsed.([]any)
	if !ok {
		return nil, errUnknownList
	}
	entries := make([]pluginEntry, 0, len(values))
	seen := make(map[string]bool, len(values))
	for _, value := range values {
		entry, err := parseEntry(value)
		if err != nil || seen[entry.Name] {
			return nil, errUnknownList
		}
		seen[entry.Name] = true
		entries = append(entries, entry)
	}
	return entries, nil
}

// Keep exact native field spelling and types. Optional identity fields may be
// absent in older listings, but null or differently cased values are not proof.
func parseEntry(value any) (pluginEntry, error) {
	object, ok := value.(map[string]any)
	if !ok {
		return pluginEntry{}, errUnknownList
	}
	fields := map[string]string{}
	for key, value := range object {
		for _, field := range []string{"name", "status", "source", "version"} {
			if shared.FoldJSONKey(key) != shared.FoldJSONKey(field) {
				continue
			}
			text, valid := value.(string)
			if key != field || !valid {
				return pluginEntry{}, errUnknownList
			}
			fields[field] = text
		}
	}
	entry := pluginEntry{Name: fields["name"], Status: fields["status"], Source: fields["source"], Version: fields["version"]}
	if strings.TrimSpace(entry.Name) == "" || (entry.Status != "installed" && entry.Status != "disabled") {
		return pluginEntry{}, errUnknownList
	}
	return entry, nil
}

func findEntry(entries []pluginEntry, name string) (pluginEntry, bool) {
	for _, entry := range entries {
		if entry.Name == name {
			return entry, true
		}
	}
	return pluginEntry{}, false
}

func expectedEntry(entry pluginEntry, activePath string) bool {
	return strings.TrimSpace(entry.Source) != "" && strings.TrimSpace(activePath) != "" && shared.SameCleanPath(entry.Source, activePath)
}

func listPlugins(ctx context.Context, env clients.Env, executable string) ([]pluginEntry, error) {
	if !shared.HasClientCLI(env, executable) {
		return nil, errUnknownList
	}
	bounded, cancel := context.WithTimeout(ctx, grokListTimeout)
	defer cancel()
	result, err := shared.RunNativeRegistry(bounded, env.Runner, legacyports.Command{
		Argv:             []string{executable, "plugin", "list", "--json"},
		StdoutLimitBytes: grokListStdoutLimitBytes,
	})
	if err != nil {
		return nil, fmt.Errorf("run Grok plugin list: %w", err)
	}
	if result.ExitCode != 0 {
		return nil, fmt.Errorf("grok plugin list exited with code %d", result.ExitCode)
	}
	return parseList(result.Stdout)
}

var _ clients.RegistryInspector = (*Adapter)(nil)

func (*Adapter) UsesNativeRegistryExecutable() bool { return true }

func (*Adapter) InspectNativeRegistry(ctx context.Context, env clients.Env, _ domain.DetectedClient, plan domain.DeliveryPlan, managed *domain.ClientBinding) (clients.RegistryFinding, error) {
	if err := ctx.Err(); err != nil {
		return clients.RegistryIndeterminate, err
	}
	if env.Runner == nil {
		return clients.RegistryIndeterminate, nil
	}
	if !shared.HasListingCLI(plan.NativeRegistryExecutable) {
		// The local plugins directory is inspected separately as the prepared
		// registry; without a CLI there is no second Grok registry to query.
		return clients.RegistryClear, nil
	}
	entries, err := listPlugins(ctx, env, plan.NativeRegistryExecutable)
	if err != nil {
		return clients.RegistryIndeterminate, err
	}
	entry, found := findEntry(entries, plan.DeclaredName)
	if !found {
		return clients.RegistryClear, nil
	}
	if managed != nil && expectedEntry(entry, plan.ActivePath) {
		return clients.RegistryExpected, nil
	}
	return clients.RegistryCollision, nil
}
