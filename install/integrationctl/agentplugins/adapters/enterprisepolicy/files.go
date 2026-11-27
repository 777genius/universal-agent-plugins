package enterprisepolicy

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"unicode"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/atomicfile"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

type Path struct {
	Layer    string
	Path     string
	Required bool
}

// Files has no global state and does not discover paths from environment.
// The composition root supplies fixed candidate locations for one operation.
type Files struct {
	Paths []Path
}

type Layer struct {
	Layer   string               `json:"layer"`
	Path    string               `json:"path"`
	Present bool                 `json:"present"`
	Policy  domain.InstallPolicy `json:"policy"`
}

type Snapshot struct {
	Layers           []Layer `json:"layers"`
	Fingerprint      string  `json:"fingerprint"`
	RestrictionCount int     `json:"restriction_count"`
}

func (f Files) Load(ctx context.Context) (Snapshot, error) {
	var snapshot Snapshot
	if len(f.Paths) > 16 {
		return snapshot, configError("too_many_layers")
	}
	for _, candidate := range f.Paths {
		if err := ctx.Err(); err != nil {
			return snapshot, err
		}
		layer, err := loadLayer(candidate)
		snapshot.Layers = append(snapshot.Layers, layer)
		if err != nil {
			return snapshot, err
		}
		if layer.Present {
			snapshot.RestrictionCount += layer.Policy.RestrictionCount()
		}
	}
	body, err := json.Marshal(snapshot.Layers)
	if err != nil {
		return snapshot, configError("fingerprint_failed")
	}
	digest := sha256.Sum256(body)
	snapshot.Fingerprint = "sha256:" + hex.EncodeToString(digest[:])
	return snapshot, nil
}

func loadLayer(candidate Path) (Layer, error) {
	layer := Layer{Layer: domain.PolicySafeText(candidate.Layer), Path: domain.PolicySafeText(candidate.Path)}
	path, err := policyPath(candidate.Path)
	if err != nil {
		return layer, layerError(err, layer)
	}
	layer.Path = path
	accessPath := policyFilesystemPath(path)
	_, err = os.Lstat(accessPath)
	if errors.Is(err, os.ErrNotExist) && !candidate.Required {
		// An absent automatic layer must not impose new directory requirements
		// on installations without policy. A dangling leaf link is present.
		return layer, nil
	}
	if err != nil {
		if parentErr := checkPolicyParent(accessPath); parentErr != nil {
			return layer, layerError(parentErr, layer)
		}
		return layer, layerError(&ConfigError{Code: "required_file_unavailable", cause: err}, layer)
	}
	layer.Present = true
	layer.Policy, err = ReadFile(path)
	return layer, layerError(err, layer)
}

func layerError(err error, layer Layer) error {
	if err == nil {
		return nil
	}
	var config *ConfigError
	if errors.As(err, &config) {
		return &ConfigError{Layer: layer.Layer, Path: layer.Path, Code: config.Code, cause: config.cause}
	}
	return &ConfigError{Layer: layer.Layer, Path: layer.Path, Code: "unavailable"}
}

func policyPath(raw string) (string, error) {
	if raw == "" || raw != strings.TrimSpace(raw) || strings.ContainsFunc(raw, unicode.IsControl) {
		return "", configError("invalid_path")
	}
	path, err := filepath.Abs(raw)
	if err != nil {
		return "", configError("invalid_path")
	}
	return path, nil
}

func policyFilesystemPath(path string) string {
	if runtime.GOOS == "darwin" {
		return darwinPolicyPath(path)
	}
	return path
}

// Only macOS's fixed system aliases have canonical policy locations. Resolving
// arbitrary links would let a user-controlled ancestor redirect policy input.
func darwinPolicyPath(path string) string {
	for _, alias := range []string{"/var", "/tmp", "/etc"} {
		if path == alias || strings.HasPrefix(path, alias+"/") {
			return "/private" + path
		}
	}
	return path
}

// ReadFile reuses the bounded stable-file reader used for dependency authority
// documents, checking every ancestor before and after the read. This refuses
// linked paths; it is not a concurrent directory-isolation or sandbox API.
func ReadFile(raw string) (domain.InstallPolicy, error) {
	path, err := policyPath(raw)
	if err != nil {
		return domain.InstallPolicy{}, err
	}
	layer := Layer{Path: path}
	path = policyFilesystemPath(path)
	if err := checkPolicyParent(path); err != nil {
		return domain.InstallPolicy{}, layerError(err, layer)
	}
	body, err := atomicfile.ReadBoundedRegularFile(path, MaxBytes)
	if err != nil {
		return domain.InstallPolicy{}, layerError(&ConfigError{Code: "unreadable_or_unsafe_file", cause: err}, layer)
	}
	if err := checkPolicyParent(path); err != nil {
		return domain.InstallPolicy{}, layerError(err, layer)
	}
	policy, err := Decode(body)
	return policy, layerError(err, layer)
}

func checkPolicyParent(path string) error {
	for parent := filepath.Dir(path); ; parent = filepath.Dir(parent) {
		next := filepath.Dir(parent)
		info, err := os.Lstat(parent)
		if err == nil {
			if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
				return configError("unsafe_parent_directory")
			}
		} else if !errors.Is(err, os.ErrNotExist) || next == parent {
			return &ConfigError{Code: "parent_directory_unavailable", cause: err}
		}
		if next == parent {
			return nil
		}
	}
}
