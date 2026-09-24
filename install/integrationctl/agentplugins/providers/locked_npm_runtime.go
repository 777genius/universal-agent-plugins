package providers

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"golang.org/x/mod/semver"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/pathpolicy"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

const (
	lockedRuntimePath = "io.github.777genius.agentplugins/runtime"
	lockedLauncherArg = "${PLUGIN_ROOT}/" + lockedRuntimePath + "/launcher.mjs"
	runtimeMarkerName = ".agentplugins-runtime.json"
)

var lockedNPMName = regexp.MustCompile(`^(?:@[a-z0-9][a-z0-9._-]*/)?[a-z0-9][a-z0-9._-]*$`)

type lockedRuntimeConfig struct {
	SchemaVersion     int    `json:"schema_version"`
	Package           string `json:"package"`
	Version           string `json:"version"`
	Entrypoint        string `json:"entrypoint"`
	PackageLockSHA256 string `json:"package_lock_sha256"`
	OmitOptional      bool   `json:"omit_optional"`
}

type lockedRuntimeMarker struct {
	SchemaVersion int    `json:"schema_version"`
	LockDigest    string `json:"lock_digest"`
	Package       string `json:"package"`
	Version       string `json:"version"`
	OmitOptional  bool   `json:"omit_optional"`
	Entrypoint    string `json:"entrypoint"`
}

// PrepareRuntime materializes a declarative, integrity-locked npm runtime before
// client activation. It never runs the plugin's launcher or npm lifecycle scripts.
// Unselected/non-runtime MCP packages remain inert.
func (manager PluginDataManager) PrepareRuntime(ctx context.Context, envelope domain.PackageEnvelope, plan domain.DeliveryPlan, dataPath string) error {
	if !selectedLockedRuntime(envelope, plan) {
		return nil
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := pathpolicy.RequireContainedChild(manager.Base, dataPath); err != nil {
		return fmt.Errorf("unsafe PLUGIN_DATA runtime locator: %w", err)
	}
	if _, err := manager.readOwned(dataPath); err != nil {
		return err
	}
	runtimeRoot := filepath.Join(envelope.SnapshotRoot, filepath.FromSlash(lockedRuntimePath))
	if err := requireRealDirectory(envelope.SnapshotRoot); err != nil {
		return fmt.Errorf("locked runtime snapshot: %w", err)
	}
	if err := requireRealDirectory(filepath.Dir(runtimeRoot)); err != nil {
		return fmt.Errorf("locked runtime parent: %w", err)
	}
	if err := requireRealDirectory(runtimeRoot); err != nil {
		return fmt.Errorf("locked runtime directory: %w", err)
	}
	configBody, err := readRuntimeFile(runtimeRoot, "runtime.json")
	if err != nil {
		return err
	}
	packageBody, err := readRuntimeFile(runtimeRoot, "package.json")
	if err != nil {
		return err
	}
	lockBody, err := readRuntimeFile(runtimeRoot, "package-lock.json")
	if err != nil {
		return err
	}
	if _, err := readRuntimeFile(runtimeRoot, "launcher.mjs"); err != nil {
		return err
	}
	config, digest, err := validateLockedRuntime(configBody, packageBody, lockBody)
	if err != nil {
		return err
	}
	runner := manager.RunNPM
	if runner == nil {
		runner = runLockedNPM
	}
	return prepareLockedRuntime(ctx, dataPath, config, digest, packageBody, lockBody, runner)
}

func selectedLockedRuntime(envelope domain.PackageEnvelope, plan domain.DeliveryPlan) bool {
	for _, name := range domain.SelectedMCPNames(plan) {
		server := envelope.MCP.Servers[name]
		if server.Type != "stdio" || server.Decoded["command"] != "node" {
			continue
		}
		if hasLockedLauncherArg(server.Decoded["args"]) {
			return true
		}
	}
	return false
}

func hasLockedLauncherArg(value any) bool {
	switch args := value.(type) {
	case []any:
		return len(args) > 0 && args[0] == lockedLauncherArg
	case []string:
		return len(args) > 0 && args[0] == lockedLauncherArg
	default:
		return false
	}
}

func requireRealDirectory(name string) error {
	info, err := os.Lstat(name)
	if err != nil {
		return err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("%s is not a real directory", name)
	}
	return nil
}

func readRuntimeFile(root, name string) ([]byte, error) {
	file := filepath.Join(root, name)
	info, err := os.Lstat(file)
	if err != nil {
		return nil, fmt.Errorf("read locked runtime %s: %w", name, err)
	}
	if !info.Mode().IsRegular() || info.Size() > 4<<20 {
		return nil, fmt.Errorf("locked runtime %s must be a regular file of at most 4 MiB", name)
	}
	body, err := os.ReadFile(file)
	if err != nil {
		return nil, fmt.Errorf("read locked runtime %s: %w", name, err)
	}
	return body, nil
}

func validateLockedRuntime(configBody, packageBody, lockBody []byte) (lockedRuntimeConfig, string, error) {
	var config lockedRuntimeConfig
	if err := decodeStrictJSON(configBody, &config); err != nil {
		return config, "", fmt.Errorf("runtime.json: %w", err)
	}
	sum := sha256.Sum256(lockBody)
	digest := "sha256:" + hex.EncodeToString(sum[:])
	if err := validateRuntimeConfig(config, digest); err != nil {
		return config, "", err
	}
	if err := validateRuntimeManifest(config, packageBody); err != nil {
		return config, "", err
	}
	if err := validateRuntimeLock(config, lockBody); err != nil {
		return config, "", err
	}
	return config, digest, nil
}

func validateRuntimeConfig(config lockedRuntimeConfig, digest string) error {
	if config.SchemaVersion != 1 || config.Package == "" || config.Version == "" ||
		config.PackageLockSHA256 != digest || strings.ContainsAny(config.Entrypoint, "\\") ||
		path.Clean(config.Entrypoint) != config.Entrypoint ||
		!strings.HasPrefix(config.Entrypoint, "node_modules/"+config.Package+"/") {
		return fmt.Errorf("runtime.json does not match the locked npm runtime")
	}
	if !lockedNPMName.MatchString(config.Package) || !semver.IsValid("v"+config.Version) {
		return fmt.Errorf("invalid locked npm package identity")
	}
	return nil
}

func validateRuntimeManifest(config lockedRuntimeConfig, packageBody []byte) error {
	var manifest struct {
		Name         string            `json:"name"`
		Version      string            `json:"version"`
		Private      bool              `json:"private"`
		Dependencies map[string]string `json:"dependencies"`
		Overrides    map[string]string `json:"overrides"`
	}
	if err := decodeStrictJSON(packageBody, &manifest); err != nil {
		return fmt.Errorf("package.json: %w", err)
	}
	if !manifest.Private || len(manifest.Dependencies) != 1 || manifest.Dependencies[config.Package] != config.Version {
		return fmt.Errorf("package.json must declare one exact private production dependency")
	}
	for name, version := range manifest.Overrides {
		if !lockedNPMName.MatchString(name) || !semver.IsValid("v"+version) {
			return fmt.Errorf("package.json contains a non-exact npm override")
		}
	}
	return nil
}

type runtimeLockPackage struct {
	Version      string            `json:"version"`
	Dependencies map[string]string `json:"dependencies"`
	Resolved     string            `json:"resolved"`
	Integrity    string            `json:"integrity"`
	Link         bool              `json:"link"`
}

func validateRuntimeLock(config lockedRuntimeConfig, lockBody []byte) error {
	var lock struct {
		LockfileVersion int                           `json:"lockfileVersion"`
		Requires        bool                          `json:"requires"`
		Packages        map[string]runtimeLockPackage `json:"packages"`
	}
	if err := json.Unmarshal(lockBody, &lock); err != nil {
		return fmt.Errorf("package-lock.json: %w", err)
	}
	root, rootOK := lock.Packages[""]
	dependency, dependencyOK := lock.Packages["node_modules/"+config.Package]
	if lock.LockfileVersion != 3 || !lock.Requires || !rootOK || !dependencyOK ||
		len(root.Dependencies) != 1 || root.Dependencies[config.Package] != config.Version ||
		dependency.Version != config.Version {
		return fmt.Errorf("package-lock.json does not bind the declared npm dependency")
	}
	for name, entry := range lock.Packages {
		if name == "" {
			continue
		}
		if err := validateRuntimeLockPackage(name, entry); err != nil {
			return err
		}
	}
	return nil
}

func validateRuntimeLockPackage(name string, entry runtimeLockPackage) error {
	if !strings.HasPrefix(name, "node_modules/") || strings.Contains(name, "..") ||
		strings.Contains(name, "\\") || entry.Link || entry.Version == "" {
		return fmt.Errorf("package-lock.json contains an unsupported package entry %q", name)
	}
	resolved, err := url.Parse(entry.Resolved)
	if err != nil || resolved.Scheme != "https" || resolved.Host != "registry.npmjs.org" ||
		resolved.User != nil || resolved.RawQuery != "" || resolved.Fragment != "" {
		return fmt.Errorf("package-lock.json package %q is not pinned to registry.npmjs.org", name)
	}
	decoded, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(entry.Integrity, "sha512-"))
	if !strings.HasPrefix(entry.Integrity, "sha512-") || err != nil || len(decoded) != sha256.Size*2 {
		return fmt.Errorf("package-lock.json package %q lacks sha512 integrity", name)
	}
	return nil
}

func decodeStrictJSON(body []byte, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return fmt.Errorf("unexpected trailing JSON")
	}
	return nil
}

func runtimeMarker(config lockedRuntimeConfig, digest string) lockedRuntimeMarker {
	return lockedRuntimeMarker{SchemaVersion: 1, LockDigest: digest, Package: config.Package,
		Version: config.Version, OmitOptional: config.OmitOptional, Entrypoint: config.Entrypoint}
}

func readyLockedRuntime(target string, expected lockedRuntimeMarker) (bool, error) {
	if _, err := os.Lstat(target); errors.Is(err, os.ErrNotExist) {
		return false, nil
	} else if err != nil {
		return false, err
	}
	if err := requireRealDirectory(target); err != nil {
		return false, err
	}
	body, err := readRuntimeFile(target, runtimeMarkerName)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	var marker lockedRuntimeMarker
	if err := decodeStrictJSON(body, &marker); err != nil {
		return false, fmt.Errorf("invalid locked npm runtime marker: %w", err)
	}
	if marker != expected {
		return false, nil
	}
	entrypoint := filepath.Join(target, filepath.FromSlash(expected.Entrypoint))
	if err := pathpolicy.RequireContainedChild(target, entrypoint); err != nil {
		return false, fmt.Errorf("unsafe locked npm entrypoint: %w", err)
	}
	info, err := os.Stat(entrypoint)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if !info.Mode().IsRegular() {
		return false, nil
	}
	binPath := filepath.Join(target, "node_modules", ".bin")
	if err := pathpolicy.RequireContainedChild(target, binPath); err != nil {
		return false, fmt.Errorf("unsafe locked npm bin directory: %w", err)
	}
	if err := requireRealDirectory(binPath); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func prepareLockedRuntime(ctx context.Context, dataPath string, config lockedRuntimeConfig, digest string, packageBody, lockBody []byte, runner func(context.Context, string, string, bool) error) (returnErr error) {
	store := filepath.Join(dataPath, "npm-runtime")
	if err := os.Mkdir(store, 0o700); err != nil && !errors.Is(err, os.ErrExist) {
		return err
	}
	if err := requireRealDirectory(store); err != nil {
		return err
	}
	key := strings.TrimPrefix(digest, "sha256:")
	target := filepath.Join(store, key)
	expected := runtimeMarker(config, digest)
	if ready, err := readyLockedRuntime(target, expected); err != nil || ready {
		return err
	}
	lockPath := target + ".lock"
	owner, err := acquireRuntimeLock(ctx, lockPath, target, expected)
	if err != nil {
		return err
	}
	if owner == "" {
		return nil // another process completed the same runtime while we waited
	}
	defer func() {
		if err := releaseRuntimeLock(lockPath, owner); err != nil {
			returnErr = errors.Join(returnErr, err)
		}
	}()
	return installLockedRuntime(ctx, dataPath, store, target, expected, config, packageBody, lockBody, runner)
}

func installLockedRuntime(ctx context.Context, dataPath, store, target string, expected lockedRuntimeMarker, config lockedRuntimeConfig, packageBody, lockBody []byte, runner func(context.Context, string, string, bool) error) error {
	if ready, err := readyLockedRuntime(target, expected); err != nil || ready {
		return err
	}
	temporary, err := os.MkdirTemp(store, ".tmp-"+strings.TrimPrefix(expected.LockDigest, "sha256:")+"-")
	if err != nil {
		return err
	}
	defer func() { _ = os.RemoveAll(temporary) }()
	if err := os.WriteFile(filepath.Join(temporary, "package.json"), packageBody, 0o600); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(temporary, "package-lock.json"), lockBody, 0o600); err != nil {
		return err
	}
	if err := runner(ctx, temporary, dataPath, config.OmitOptional); err != nil {
		return err
	}
	if err := validatePreparedRuntime(temporary, config.Entrypoint); err != nil {
		return err
	}
	if err := writeRuntimeMarker(temporary, expected); err != nil {
		return err
	}
	if err := commitLockedRuntimeTarget(store, target, temporary, expected); err != nil {
		return err
	}
	ready, err := readyLockedRuntime(target, expected)
	if err != nil {
		return fmt.Errorf("prepared npm runtime failed its ready check: %w", err)
	}
	if !ready {
		return fmt.Errorf("prepared npm runtime failed its ready check")
	}
	return nil
}

func validatePreparedRuntime(temporary, entrypointName string) error {
	entrypoint := filepath.Join(temporary, filepath.FromSlash(entrypointName))
	if err := pathpolicy.RequireContainedChild(temporary, entrypoint); err != nil {
		return fmt.Errorf("unsafe locked npm entrypoint: %w", err)
	}
	info, err := os.Stat(entrypoint)
	if err != nil || !info.Mode().IsRegular() {
		return fmt.Errorf("locked npm package did not provide %s", entrypointName)
	}
	binPath := filepath.Join(temporary, "node_modules", ".bin")
	if err := pathpolicy.RequireContainedChild(temporary, binPath); err != nil {
		return fmt.Errorf("unsafe locked npm bin directory: %w", err)
	}
	if err := requireRealDirectory(binPath); err != nil {
		return fmt.Errorf("locked npm package did not provide node_modules/.bin: %w", err)
	}
	return nil
}

func writeRuntimeMarker(temporary string, expected lockedRuntimeMarker) error {
	body, err := json.Marshal(expected)
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(temporary, runtimeMarkerName), append(body, '\n'), 0o600); err != nil {
		return err
	}
	return nil
}

func acquireRuntimeLock(ctx context.Context, lockPath, target string, expected lockedRuntimeMarker) (string, error) {
	deadline := time.NewTimer(12 * time.Minute)
	defer deadline.Stop()
	for {
		if err := os.Mkdir(lockPath, 0o700); err == nil {
			return recordRuntimeLockOwner(lockPath)
		} else if !errors.Is(err, os.ErrExist) {
			return "", err
		}
		if ready, err := readyLockedRuntime(target, expected); err != nil || ready {
			return "", err
		}
		if reclaimed, err := reclaimRuntimeLock(lockPath); err != nil {
			return "", err
		} else if reclaimed {
			continue
		}
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-deadline.C:
			return "", fmt.Errorf("locked npm runtime is being prepared by another process; retry after it finishes")
		case <-time.After(250 * time.Millisecond):
		}
	}
}

func releaseRuntimeLock(lockPath, owner string) error {
	before, err := snapshotRuntimeLock(lockPath)
	if err != nil {
		return err
	}
	if before == nil || string(before.owner) != owner {
		return fmt.Errorf("locked npm runtime ownership changed before lock release")
	}
	digest, err := runtimeLockRetirementDigest(lockPath)
	if err != nil {
		return err
	}
	if digest == "" {
		return fmt.Errorf("locked npm runtime disappeared before lock release")
	}
	same, err := sameRuntimeLock(lockPath, before)
	if err != nil {
		return err
	}
	if !same {
		return fmt.Errorf("locked npm runtime ownership changed during lock release")
	}
	return os.Rename(lockPath, lockPath+".retired-"+digest)
}
