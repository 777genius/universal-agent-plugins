package nativeconfig

import (
	"fmt"
	"sort"
	"strings"
)

// WriterLockPaths returns the exact lockfile identities used by the default
// native MCP writer, sorted by path. JSON is required; JSONC is optional. Both
// candidates are locked even when only one exists, so file selection and the
// ambiguous-config check happen under the locks. Paths must be absolute, clean
// and distinct, as for Apply; the codec must be supported.
//
// This function performs no filesystem access, creates no locks and does not
// resolve symlinks or select a profile. Callers must validate their authority
// over the supplied paths. Do not append a suffix to the returned identities.
// Cline uses populated-directory .lock locks; all other codecs use regular-file
// .agentplugins.lock locks. This describes the default writer, not an injected
// NewWithLockAcquirer implementation, and does not acquire a lock on its behalf.
func WriterLockPaths(paths Paths, codec Codec) ([]string, error) {
	targets, err := writerLockTargets(paths, codec)
	if err != nil {
		return nil, err
	}
	locks := make([]string, len(targets))
	for index, target := range targets {
		locks[index] = target.lockPath
	}
	sort.Strings(locks)
	return locks, nil
}

type writerLockTarget struct {
	configPath string
	lockPath   string
}

func writerLockTargets(paths Paths, codec Codec) ([]writerLockTarget, error) {
	if err := validateExactPath(paths.JSON, "JSON native config path"); err != nil {
		return nil, err
	}
	candidates := []string{paths.JSON}
	if paths.JSONC != "" {
		if err := validateExactPath(paths.JSONC, "JSONC native config path"); err != nil {
			return nil, err
		}
		if paths.JSON == paths.JSONC {
			return nil, fmt.Errorf("JSON and JSONC native config paths must differ")
		}
		candidates = append(candidates, paths.JSONC)
	}
	if !supportedCodec(codec) {
		return nil, fmt.Errorf("unsupported native config codec %q", codec)
	}
	suffix := ".agentplugins.lock"
	if codec == CodecCline {
		suffix = ".lock"
	}
	targets := make([]writerLockTarget, len(candidates))
	for index, path := range candidates {
		if strings.ContainsRune(path, 0) {
			return nil, fmt.Errorf("native config path must not contain NUL")
		}
		targets[index] = writerLockTarget{configPath: path, lockPath: path + suffix}
	}
	// Keep the existing writer's candidate acquisition order and process-mutex
	// keys. Exporting identities must not change its locking protocol.
	sort.Slice(targets, func(i, j int) bool { return targets[i].configPath < targets[j].configPath })
	return targets, nil
}
