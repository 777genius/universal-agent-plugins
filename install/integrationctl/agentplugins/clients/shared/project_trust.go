package shared

import (
	"bytes"
	"errors"
	"fmt"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
)

// UpdateProjectTrustFile records a confirmed installation's exact-project trust
// preference. Each client owns its document format and policy rules. Trust is a
// persistent user preference, not a plugin-owned entry removed at uninstall.
// Call only during confirmed activation, never during planning or verification.
func UpdateProjectTrustFile(kernel nativeconfig.Kernel, path string, mutate func([]byte) ([]byte, error)) (changed bool, err error) {
	return UpdateProjectTrustFileWithInitial(kernel, path, []byte("{}"), mutate)
}

// UpdateProjectTrustFileWithInitial supports clients whose absent document is
// not an empty JSON object, such as Codex's empty TOML configuration.
func UpdateProjectTrustFileWithInitial(kernel nativeconfig.Kernel, path string, initial []byte, mutate func([]byte) ([]byte, error)) (changed bool, err error) {
	if mutate == nil {
		return false, fmt.Errorf("project trust mutation is unavailable")
	}
	file, err := kernel.BeginExactFile(path)
	if err != nil {
		return false, err
	}
	defer func() { err = errors.Join(err, file.Close()) }()
	original := file.Original()
	if len(original.Body) > 1<<20 {
		return false, fmt.Errorf("project trust document exceeds limit")
	}
	body := original.Body
	if !original.Exists {
		body = bytes.Clone(initial)
	}
	next, err := mutate(bytes.Clone(body))
	if err != nil {
		return false, err
	}
	if bytes.Equal(body, next) {
		return false, nil
	}
	if len(next) == 0 || len(next) > 1<<20 {
		return false, fmt.Errorf("project trust mutation exceeds document limits")
	}
	if err := file.Apply(next); err != nil {
		return false, errors.Join(err, file.Rollback())
	}
	return true, nil
}
