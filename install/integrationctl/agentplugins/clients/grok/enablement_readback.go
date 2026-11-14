package grok

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

func (recovery EnablementRecovery) verifyRecordConfig(digest string, mode uint32) error {
	current, err := recovery.Kernel.ReadExactFile(filepath.Join(recovery.UserRoot, "config.toml"))
	if err != nil || !current.Exists || current.Mode != os.FileMode(mode) || enablementDigest(current.Body) != digest {
		return errors.Join(err, fmt.Errorf("grok native config changed after its exact membership edit"))
	}
	return nil
}
