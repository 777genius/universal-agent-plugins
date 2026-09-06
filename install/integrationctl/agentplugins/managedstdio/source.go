package managedstdio

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Source is explicitly created by the composition root from its own executable.
// Its private digest binds subsequent copies to those exact trusted bytes.
type Source struct{ path, digest, version string }

func NewSource(executable, version string) (*Source, error) {
	if !filepath.IsAbs(executable) {
		return nil, fmt.Errorf("launcher source must be absolute")
	}
	body, err := readExecutable(executable)
	if err != nil {
		return nil, err
	}
	hash := sha256.Sum256(body)
	return &Source{path: executable, digest: hex.EncodeToString(hash[:]), version: version}, nil
}
func readExecutable(path string) ([]byte, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Mode().Perm()&0111 == 0 {
		return nil, fmt.Errorf("launcher source is not executable")
	}
	return os.ReadFile(path)
}
func (source *Source) Available() error {
	if source == nil {
		return fmt.Errorf("trusted launcher source is unavailable")
	}
	body, err := readExecutable(source.path)
	if err != nil {
		return err
	}
	hash := sha256.Sum256(body)
	if hex.EncodeToString(hash[:]) != source.digest {
		return fmt.Errorf("trusted launcher source changed")
	}
	return nil
}

// CheckCollision never follows an authored namespace symlink or overwrites files.
func CheckCollision(root string) (bool, error) {
	parent := filepath.Join(root, "io.github.777genius.agentplugins")
	info, err := os.Lstat(parent)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return true, nil
	}
	_, err = os.Lstat(filepath.Join(root, filepath.FromSlash(RelativeDirectory)))
	if os.IsNotExist(err) {
		return false, nil
	}
	return err == nil, err
}
func (source *Source) Deliver(root string) error {
	collision, err := CheckCollision(root)
	if err != nil {
		return err
	}
	if collision {
		return fmt.Errorf("managed launcher reserved path collision")
	}
	if err := source.Available(); err != nil {
		return err
	}
	body, err := readExecutable(source.path)
	if err != nil {
		return err
	}
	hash := sha256.Sum256(body)
	if hex.EncodeToString(hash[:]) != source.digest {
		return fmt.Errorf("trusted launcher source changed")
	}
	directory := filepath.Join(root, filepath.FromSlash(RelativeDirectory))
	if err := os.MkdirAll(directory, 0700); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(directory, ExecutableName), body, 0700); err != nil {
		return err
	}
	metadata, err := json.Marshal(struct {
		Protocol int    `json:"protocol"`
		SHA256   string `json:"sha256"`
		Version  string `json:"cliVersion"`
	}{1, source.digest, source.version})
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(directory, "metadata.json"), metadata, 0600)
}
