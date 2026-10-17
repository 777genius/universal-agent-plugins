package providers

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

type kimiInstalledFile struct {
	Version int                   `json:"version"`
	Plugins []kimiInstalledRecord `json:"plugins"`
}

type kimiInstalledRecord struct {
	ID             string `json:"id"`
	Root           string `json:"root"`
	Source         string `json:"source"`
	Enabled        bool   `json:"enabled"`
	InstalledAt    string `json:"installedAt"`
	UpdatedAt      string `json:"updatedAt,omitempty"`
	OriginalSource string `json:"originalSource,omitempty"`
}

func kimiInstalledPath(configRoot string) string {
	return filepath.Join(configRoot, "plugins", "installed.json")
}

func inspectKimiRegistry(plan domain.DeliveryPlan, managed *domain.ClientBinding) (registryFinding, error) {
	root := strings.TrimSpace(plan.NativeRegistryRoot)
	if root == "" {
		return registryIndeterminate, nil
	}
	file, err := readKimiInstalled(root)
	if err != nil {
		if os.IsNotExist(err) {
			return registryClear, nil
		}
		return registryIndeterminate, err
	}
	id := strings.TrimSpace(plan.DeclaredName)
	active := strings.TrimSpace(plan.ActivePath)
	for _, record := range file.Plugins {
		if strings.TrimSpace(record.ID) != id {
			continue
		}
		if samePath(record.Root, active) {
			if managed != nil {
				return registryExpected, nil
			}
			return registryCollision, nil
		}
		return registryCollision, nil
	}
	return registryClear, nil
}

func upsertKimiInstalledRecord(configRoot, pluginID, pluginRoot string) error {
	id := strings.TrimSpace(pluginID)
	root := strings.TrimSpace(pluginRoot)
	if id == "" || root == "" || strings.TrimSpace(configRoot) == "" {
		return fmt.Errorf("Kimi installed.json identity is incomplete")
	}
	file, err := readKimiInstalled(configRoot)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if file.Version == 0 {
		file.Version = 1
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	updated := false
	for index, record := range file.Plugins {
		if strings.TrimSpace(record.ID) != id {
			continue
		}
		if !samePath(record.Root, root) && strings.TrimSpace(record.Root) != "" {
			return fmt.Errorf("Kimi plugin %q is already installed from %s", id, record.Root)
		}
		record.Root = root
		record.Source = "local-path"
		record.Enabled = true
		record.UpdatedAt = now
		if strings.TrimSpace(record.InstalledAt) == "" {
			record.InstalledAt = now
		}
		file.Plugins[index] = record
		updated = true
		break
	}
	if !updated {
		file.Plugins = append(file.Plugins, kimiInstalledRecord{
			ID:          id,
			Root:        root,
			Source:      "local-path",
			Enabled:     true,
			InstalledAt: now,
			UpdatedAt:   now,
		})
	}
	if err := os.MkdirAll(filepath.Dir(kimiInstalledPath(configRoot)), 0o700); err != nil {
		return fmt.Errorf("create Kimi plugin registry directory: %w", err)
	}
	return writeJSON(kimiInstalledPath(configRoot), file)
}

func removeKimiInstalledRecord(configRoot, pluginID, pluginRoot string) error {
	id := strings.TrimSpace(pluginID)
	if id == "" || strings.TrimSpace(configRoot) == "" {
		return nil
	}
	file, err := readKimiInstalled(configRoot)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	kept := file.Plugins[:0]
	for _, record := range file.Plugins {
		if strings.TrimSpace(record.ID) != id {
			kept = append(kept, record)
			continue
		}
		if pluginRoot != "" && !samePath(record.Root, pluginRoot) {
			kept = append(kept, record)
		}
	}
	if len(kept) == len(file.Plugins) {
		return nil
	}
	file.Plugins = kept
	return writeJSON(kimiInstalledPath(configRoot), file)
}

func readKimiInstalled(configRoot string) (kimiInstalledFile, error) {
	body, err := os.ReadFile(kimiInstalledPath(configRoot))
	if err != nil {
		return kimiInstalledFile{}, err
	}
	var file kimiInstalledFile
	if err := json.Unmarshal(body, &file); err != nil {
		return kimiInstalledFile{}, fmt.Errorf("parse Kimi installed.json: %w", err)
	}
	if file.Plugins == nil {
		file.Plugins = []kimiInstalledRecord{}
	}
	return file, nil
}

func samePath(left, right string) bool {
	left = strings.TrimSpace(left)
	right = strings.TrimSpace(right)
	if left == "" || right == "" {
		return false
	}
	leftAbs, err := filepath.Abs(left)
	if err != nil {
		return false
	}
	rightAbs, err := filepath.Abs(right)
	if err != nil {
		return false
	}
	return filepath.Clean(leftAbs) == filepath.Clean(rightAbs)
}
