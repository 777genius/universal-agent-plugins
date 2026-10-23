package sourceacquisition

import (
	"errors"
	"fmt"
	"path"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/pathpolicy"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"golang.org/x/text/cases"
	"golang.org/x/text/unicode/norm"
)

type archivePath struct {
	name      string
	directory bool
	explicit  bool
}

type archivePolicy struct {
	entries int
	bytes   int64
	paths   map[string]archivePath
}

func newArchivePolicy() *archivePolicy {
	return &archivePolicy{paths: make(map[string]archivePath)}
}

func (policy *archivePolicy) entry(raw string, directory bool, size int64) (string, error) {
	policy.entries++
	if policy.entries > domain.DefaultMaxFiles {
		return "", errors.New("archive exceeds 10000 entries")
	}
	if size < 0 || size > domain.DefaultMaxFileBytes || size > archiveMaxBytes-policy.bytes {
		return "", errors.New("archive exceeds file (64 MiB) or expanded byte (256 MiB) limits")
	}
	if directory && size != 0 {
		return "", errors.New("archive directory has nonzero size")
	}
	policy.bytes += size
	// A conventional root directory marker is the sole normalization allowance.
	if directory && (raw == "./" || raw == ".") {
		return "", nil
	}
	name := raw
	if directory {
		name = strings.TrimSuffix(name, "/")
	}
	if name == "" || !utf8.ValidString(name) || !norm.NFC.IsNormalString(name) || path.IsAbs(name) || path.Clean(name) != name || strings.ContainsFunc(name, unicode.IsControl) {
		return "", fmt.Errorf("archive contains non-portable path %q", raw)
	}
	parts := strings.Split(name, "/")
	if len(parts) > domain.DefaultMaxDepth {
		return "", errors.New("archive path exceeds depth 64")
	}
	for _, part := range parts {
		if err := pathpolicy.ValidatePortablePathSegment(part); err != nil || strings.EqualFold(part, ".git") {
			return "", fmt.Errorf("archive contains unsafe path %q", raw)
		}
	}
	for index := range parts {
		current := strings.Join(parts[:index+1], "/")
		key := cases.Fold().String(current)
		prior, found := policy.paths[key]
		leaf := index == len(parts)-1
		if found && prior.name != current {
			return "", fmt.Errorf("archive paths %q and %q collide by case or Unicode", prior.name, current)
		}
		if found && !prior.directory && (!leaf || directory) || leaf && found && (prior.explicit || prior.directory && !directory) {
			return "", fmt.Errorf("archive has duplicate or file/directory conflict at %q", current)
		}
		if leaf {
			policy.paths[key] = archivePath{name: current, directory: directory, explicit: true}
		} else if !found {
			policy.paths[key] = archivePath{name: current, directory: true}
		}
	}
	return name, nil
}

func archiveZero(body []byte) bool {
	for _, value := range body {
		if value != 0 {
			return false
		}
	}
	return true
}
