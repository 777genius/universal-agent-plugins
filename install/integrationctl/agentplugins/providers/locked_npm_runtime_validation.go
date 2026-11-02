package providers

import (
	"bytes"
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
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/pathpolicy"
	"golang.org/x/mod/semver"
)

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
