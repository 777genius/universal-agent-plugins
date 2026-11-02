package kimi

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/atomicfile"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/pathpolicy"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
)

// Keep the complete document so capabilities, GitHub metadata and future fields
// survive updates, including fields on the managed record itself.
type registry struct {
	document map[string]any
	records  []any
	original []byte
}

func registryPath(root string) string { return filepath.Join(root, "plugins", "installed.json") }
func validateRegistryPath(root string) error {
	if !filepath.IsAbs(root) {
		return fmt.Errorf("kimi configuration root must be absolute")
	}
	return pathpolicy.RequireContainedChild(root, registryPath(root))
}
func readRegistry(root string) (registry, error) {
	if err := validateRegistryPath(root); err != nil {
		return registry{}, err
	}
	path := registryPath(root)
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return registry{document: map[string]any{"version": json.Number("1"), "plugins": []any{}}, records: []any{}}, nil
	}
	if err != nil {
		return registry{}, err
	}
	if !info.Mode().IsRegular() || info.Size() > 4*1024*1024 {
		return registry{}, fmt.Errorf("invalid or oversized Kimi registry")
	}
	body, err := os.ReadFile(path)
	if err != nil {
		return registry{}, err
	}
	doc, err := shared.DecodeStrictJSONObject(body)
	if err != nil {
		return registry{}, err
	}
	if doc["version"] != json.Number("1") {
		return registry{}, fmt.Errorf("unsupported Kimi registry version")
	}
	records, ok := doc["plugins"].([]any)
	if !ok {
		return registry{}, fmt.Errorf("kimi registry plugins must be an array")
	}
	ids := map[string]bool{}
	for _, raw := range records {
		record, ok := raw.(map[string]any)
		if !ok {
			return registry{}, fmt.Errorf("invalid Kimi registry record")
		}
		id, ok := record["id"].(string)
		if !ok || strings.TrimSpace(id) == "" || ids[strings.ToLower(id)] {
			return registry{}, fmt.Errorf("invalid or duplicate Kimi registry identity")
		}
		ids[strings.ToLower(id)] = true
		path, ok := record["root"].(string)
		if !ok || !filepath.IsAbs(path) {
			return registry{}, fmt.Errorf("invalid Kimi registry plugin root")
		}
		if _, ok := record["enabled"].(bool); !ok {
			return registry{}, fmt.Errorf("invalid Kimi enabled state")
		}
	}
	return registry{document: doc, records: records, original: body}, nil
}
func (r registry) record(id, root string) (map[string]any, error) {
	var found map[string]any
	for _, raw := range r.records {
		record := raw.(map[string]any)
		recordID := record["id"].(string)
		sameRoot := shared.SameCleanPath(record["root"].(string), root)
		if strings.EqualFold(recordID, id) {
			if recordID != id || !sameRoot {
				return nil, fmt.Errorf("kimi plugin identity collision for %q", id)
			}
			found = record
		} else if sameRoot {
			return nil, fmt.Errorf("kimi plugin root is claimed by %q", recordID)
		}
	}
	return found, nil
}

var registryMu sync.Mutex

// Serialize cooperating writers and compare the observed bytes just before the
// atomic replacement. Kimi itself does not honor this lock; like its own writer,
// a last-instant concurrent external replacement cannot be made portable CAS.
func mutateRegistry(root, id, active string, remove, replace bool, now time.Time) error {
	registryMu.Lock()
	defer registryMu.Unlock()
	if err := validateIdentity(root, id, active); err != nil {
		return err
	}
	if remove {
		if _, err := os.Lstat(registryPath(root)); os.IsNotExist(err) {
			return nil
		}
	}
	if err := os.MkdirAll(filepath.Dir(registryPath(root)), 0o700); err != nil {
		return err
	}
	lock := registryPath(root) + ".agentplugins.lock"
	file, err := acquireRegistryLock(lock)
	if err != nil {
		return err
	}
	_ = file.Close()
	defer func() { _ = os.Remove(lock) }()
	current, err := readRegistry(root)
	if err != nil {
		return err
	}
	if err := applyRegistryMutation(&current, id, active, remove, replace, now); err != nil {
		return err
	}
	body, err := json.MarshalIndent(current.document, "", "  ")
	if err != nil {
		return err
	}
	latest, err := readRegistry(root)
	if err != nil {
		return err
	}
	if !bytes.Equal(latest.original, current.original) {
		return fmt.Errorf("kimi registry changed concurrently")
	}
	return atomicfile.Write(registryPath(root), append(body, '\n'), 0o600)
}

func acquireRegistryLock(lock string) (*os.File, error) {
	file, err := os.OpenFile(lock, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if os.IsExist(err) {
		info, statErr := os.Lstat(lock)
		if statErr == nil && info.Mode().IsRegular() && time.Since(info.ModTime()) > 2*time.Minute {
			if removeErr := os.Remove(lock); removeErr != nil && !os.IsNotExist(removeErr) {
				return nil, fmt.Errorf("remove stale kimi registry lock %s: %w", lock, removeErr)
			}
			file, err = os.OpenFile(lock, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
		}
	}
	if err != nil {
		return nil, fmt.Errorf("lock kimi registry at %s (remove it if no other installation is running): %w", lock, err)
	}
	return file, nil
}

func applyRegistryMutation(current *registry, id, active string, remove, replace bool, now time.Time) error {
	record, err := current.record(id, active)
	if err != nil {
		return err
	}
	if remove {
		if record == nil {
			return nil
		}
		kept := make([]any, 0, len(current.records)-1)
		for _, raw := range current.records {
			if raw.(map[string]any)["id"] != id {
				kept = append(kept, raw)
			}
		}
		current.document["plugins"] = kept
	} else {
		if record != nil && !replace {
			return fmt.Errorf("kimi registry entry already exists without managed ownership")
		}
		stamp := now.UTC().Format(time.RFC3339Nano)
		if record == nil {
			record = map[string]any{"id": id, "root": active, "source": "local-path", "enabled": true, "installedAt": stamp}
			current.records = append(current.records, record)
		}
		// Preserve enabled/capability choices on replacement.
		record["updatedAt"] = stamp
		current.document["plugins"] = current.records
	}
	return nil
}
func validateIdentity(root, id, active string) error {
	if err := validateRegistryPath(root); err != nil {
		return err
	}
	if strings.TrimSpace(id) != id {
		return fmt.Errorf("invalid Kimi plugin identity")
	}
	if err := pathpolicy.ValidateLeafID(id); err != nil {
		return err
	}
	if !filepath.IsAbs(active) {
		return fmt.Errorf("kimi managed plugin root must be absolute")
	}
	base := filepath.Join(root, "plugins", "managed")
	if filepath.Dir(filepath.Clean(active)) != filepath.Clean(base) {
		return fmt.Errorf("kimi plugin is outside managed directory")
	}
	return pathpolicy.RequireContainedChild(root, active)
}
