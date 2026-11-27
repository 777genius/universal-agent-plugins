package enterprisepolicy

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
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
	if err := checkPolicyParent(path); err != nil {
		return layer, layerError(err, layer)
	}
	_, err = os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) && !candidate.Required {
		return layer, nil
	}
	if err != nil {
		return layer, layerError(configError("required_file_unavailable"), layer)
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
		return &ConfigError{Layer: layer.Layer, Path: layer.Path, Code: config.Code}
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

// ReadFile reuses the bounded stable-file reader used for dependency authority
// documents. Neither a symlink nor a FIFO can become executable policy input.
func ReadFile(raw string) (domain.InstallPolicy, error) {
	path, err := policyPath(raw)
	if err != nil {
		return domain.InstallPolicy{}, err
	}
	layer := Layer{Path: path}
	if err := checkPolicyParent(path); err != nil {
		return domain.InstallPolicy{}, layerError(err, layer)
	}
	body, err := atomicfile.ReadBoundedRegularFile(path, MaxBytes)
	if err != nil {
		return domain.InstallPolicy{}, layerError(configError("unreadable_or_unsafe_file"), layer)
	}
	policy, err := Decode(body)
	return policy, layerError(err, layer)
}

func checkPolicyParent(path string) error {
	for parent := filepath.Dir(path); ; parent = filepath.Dir(parent) {
		info, err := os.Lstat(parent)
		if err == nil {
			if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
				return configError("unsafe_parent_directory")
			}
			return nil
		}
		if !errors.Is(err, os.ErrNotExist) || filepath.Dir(parent) == parent {
			return configError("parent_directory_unavailable")
		}
	}
}
