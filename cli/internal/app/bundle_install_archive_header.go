package app

import (
	"archive/tar"
	"fmt"
	"path"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/pathpolicy"
	"golang.org/x/text/cases"
)

const (
	maxBundleArchiveEntries       = 10_000
	maxBundleArchiveFileBytes     = int64(64 << 20)
	maxBundleArchiveExpandedBytes = int64(256 << 20)
	maxBundleArchiveDepth         = 64
)

type bundleArchivePolicy struct {
	entries       int
	expandedBytes int64
	paths         map[string]bundleArchivePath
}

type bundleArchivePath struct {
	name      string
	directory bool
	explicit  bool
}

func newBundleArchivePolicy() *bundleArchivePolicy {
	return &bundleArchivePolicy{paths: map[string]bundleArchivePath{}}
}

func (policy *bundleArchivePolicy) validate(header *tar.Header) (string, error) {
	policy.entries++
	if policy.entries > maxBundleArchiveEntries {
		return "", fmt.Errorf("bundle install refuses archives with more than %d entries", maxBundleArchiveEntries)
	}
	name, err := validateBundleHeader(header)
	if err != nil {
		return "", err
	}
	parts := strings.Split(name, "/")
	if len(parts) > maxBundleArchiveDepth {
		return "", fmt.Errorf("bundle install refuses paths deeper than %d entries", maxBundleArchiveDepth)
	}
	if err := policy.registerPath(name, header.Typeflag == tar.TypeDir); err != nil {
		return "", err
	}
	if header.Size < 0 || header.Size > maxBundleArchiveFileBytes {
		return "", fmt.Errorf("bundle install refuses file %s larger than %d bytes", name, maxBundleArchiveFileBytes)
	}
	if header.Size > maxBundleArchiveExpandedBytes-policy.expandedBytes {
		return "", fmt.Errorf("bundle install refuses expanded archives larger than %d bytes", maxBundleArchiveExpandedBytes)
	}
	policy.expandedBytes += header.Size
	if header.Typeflag == tar.TypeDir && header.Size != 0 {
		return "", fmt.Errorf("bundle install refuses directory %s with non-zero size", name)
	}
	return name, nil
}

func (policy *bundleArchivePolicy) registerPath(name string, directory bool) error {
	parts := strings.Split(name, "/")
	current := ""
	for index, part := range parts {
		if current == "" {
			current = part
		} else {
			current += "/" + part
		}
		key := cases.Fold().String(current)
		existing, found := policy.paths[key]
		if found && existing.name != current {
			return fmt.Errorf("bundle install refuses duplicate or case-colliding entries %q and %q", existing.name, current)
		}
		leaf := index == len(parts)-1
		if found && !existing.directory && (!leaf || directory) {
			return fmt.Errorf("bundle install refuses file/directory conflict at %q", current)
		}
		if !leaf {
			if !found {
				policy.paths[key] = bundleArchivePath{name: current, directory: true}
			}
			continue
		}
		if found && existing.explicit {
			return fmt.Errorf("bundle install refuses duplicate or case-colliding entries %q and %q", existing.name, current)
		}
		if found && existing.directory && !directory {
			return fmt.Errorf("bundle install refuses file/directory conflict at %q", current)
		}
		policy.paths[key] = bundleArchivePath{name: current, directory: directory, explicit: true}
	}
	return nil
}

func validateBundleHeader(hdr *tar.Header) (string, error) {
	if err := validateBundleHeaderType(hdr); err != nil {
		return "", err
	}
	raw := hdr.Name
	if hdr.Typeflag == tar.TypeDir {
		raw = strings.TrimSuffix(raw, "/")
	}
	for _, segment := range strings.Split(raw, "/") {
		if segment == ".." {
			return "", fmt.Errorf("bundle install refuses path traversal entry %q", hdr.Name)
		}
	}
	name := path.Clean(raw)
	if raw == "" || name == "." || name != raw || path.IsAbs(raw) ||
		strings.ContainsAny(raw, "\\\x00") {
		return "", fmt.Errorf("bundle install refuses invalid archive path %q", hdr.Name)
	}
	for _, segment := range strings.Split(name, "/") {
		if err := pathpolicy.ValidatePortablePathSegment(segment); err != nil {
			return "", fmt.Errorf("bundle install refuses non-portable archive path %q: %w", hdr.Name, err)
		}
	}
	return name, nil
}

func validateBundleHeaderType(hdr *tar.Header) error {
	switch hdr.Typeflag {
	case tar.TypeReg, tar.TypeDir:
		return nil
	case tar.TypeSymlink, tar.TypeLink:
		return fmt.Errorf("bundle install refuses symlink entry %s", hdr.Name)
	default:
		return fmt.Errorf("bundle install refuses unsupported archive entry %s", hdr.Name)
	}
}
