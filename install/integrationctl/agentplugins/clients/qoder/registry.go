package qoder

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
	qoderListTimeout          = 15 * time.Second
	qoderListStdoutLimitBytes = 1 << 20
)

var errUnknownList = errors.New("qoder plugin list JSON contract is not recognized")

// pluginEntry is one `qoder plugins list --json` record. Qoder identifies an
// installed plugin by a qualified id: "name@local" for a local install,
// "name@marketplace" for a marketplace one. The version member is optional.
type pluginEntry struct {
	ID      string
	Version string
}

func localPluginID(name string) string { return name + "@local" }

func pluginNameFromID(id string) (string, bool) {
	split := strings.LastIndexByte(id, '@')
	if split <= 0 || split == len(id)-1 {
		return "", false
	}
	return id[:split], true
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
		if err != nil || seen[entry.ID] {
			return nil, errUnknownList
		}
		seen[entry.ID] = true
		entries = append(entries, entry)
	}
	return entries, nil
}

// Keep exact native field spelling and types. The version member may be absent
// in older listings, but null or differently cased values are not proof.
func parseEntry(value any) (pluginEntry, error) {
	object, ok := value.(map[string]any)
	if !ok {
		return pluginEntry{}, errUnknownList
	}
	fields := map[string]string{}
	for key, value := range object {
		for _, field := range []string{"id", "version"} {
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
	entry := pluginEntry{ID: fields["id"], Version: fields["version"]}
	if _, ok := pluginNameFromID(entry.ID); !ok {
		return pluginEntry{}, errUnknownList
	}
	return entry, nil
}

func findEntry(entries []pluginEntry, name string) (pluginEntry, bool) {
	for _, entry := range entries {
		if entry.ID == localPluginID(name) {
			return entry, true
		}
	}
	return pluginEntry{}, false
}

// findForeign reports a claim on the same plugin name under a different id,
// such as a marketplace installation. `qoder plugins uninstall` addresses a
// plugin by name, so two claims on one name are never ours to mutate.
func findForeign(entries []pluginEntry, name string) bool {
	for _, entry := range entries {
		if entryName, ok := pluginNameFromID(entry.ID); ok && entryName == name && entry.ID != localPluginID(name) {
			return true
		}
	}
	return false
}

// versionMatches accepts a listing without a version member but never a
// different one: only the exact declared version proves the delivered package.
func versionMatches(entry pluginEntry, declared string) bool {
	return entry.Version == "" || entry.Version == declared
}

func listPlugins(ctx context.Context, env clients.Env, executable string) ([]pluginEntry, error) {
	if !shared.HasClientCLI(env, executable) {
		return nil, errUnknownList
	}
	bounded, cancel := context.WithTimeout(ctx, qoderListTimeout)
	defer cancel()
	result, err := shared.RunNativeRegistry(bounded, env.Runner, legacyports.Command{
		Argv:             []string{executable, "plugins", "list", "--json"},
		StdoutLimitBytes: qoderListStdoutLimitBytes,
	})
	if err != nil {
		return nil, fmt.Errorf("run Qoder plugin list: %w", err)
	}
	if result.ExitCode != 0 {
		return nil, fmt.Errorf("qoder plugins list exited with code %d", result.ExitCode)
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
		// The managed package directory is inspected separately as the prepared
		// registry; without a CLI there is no second Qoder registry to query.
		return clients.RegistryClear, nil
	}
	entries, err := listPlugins(ctx, env, plan.NativeRegistryExecutable)
	if err != nil {
		return clients.RegistryIndeterminate, err
	}
	if findForeign(entries, plan.DeclaredName) {
		return clients.RegistryCollision, nil
	}
	if _, found := findEntry(entries, plan.DeclaredName); !found {
		return clients.RegistryClear, nil
	}
	// The list output carries no source path, so a managed binding plus the
	// exact local id is the strongest ownership evidence the CLI offers.
	if managed != nil {
		return clients.RegistryExpected, nil
	}
	return clients.RegistryCollision, nil
}
