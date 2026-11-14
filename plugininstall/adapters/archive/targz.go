package archive

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"path"
	"strings"

	"github.com/777genius/plugin-kit-ai/plugininstall/domain"
	"github.com/777genius/plugin-kit-ai/plugininstall/ports"
)

// TarGzExtractor extracts a single plugin binary from the root of a .tar.gz (GoReleaser layout).
type TarGzExtractor struct{}

var _ ports.ArchiveExtractor = (*TarGzExtractor)(nil)

var skipRootNames = map[string]struct{}{
	"readme": {}, "readme.md": {}, "license": {}, "copying": {},
}

const (
	maxArchiveEntries       = 10_000
	maxArchiveFileBytes     = int64(256 << 20)
	maxArchiveExpandedBytes = int64(512 << 20)
	maxArchivePathDepth     = 64
)

var windowsReservedArchiveNames = map[string]struct{}{
	"CON": {}, "PRN": {}, "AUX": {}, "NUL": {}, "CLOCK$": {},
	"COM1": {}, "COM2": {}, "COM3": {}, "COM4": {}, "COM5": {}, "COM6": {}, "COM7": {}, "COM8": {}, "COM9": {},
	"LPT1": {}, "LPT2": {}, "LPT3": {}, "LPT4": {}, "LPT5": {}, "LPT6": {}, "LPT7": {}, "LPT8": {}, "LPT9": {},
}

type archivePolicy struct {
	entries       int
	expandedBytes int64
	paths         map[string]archivePath
}

type archivePath struct {
	name      string
	directory bool
	explicit  bool
}

func newArchivePolicy() *archivePolicy {
	return &archivePolicy{paths: map[string]archivePath{}}
}

func (policy *archivePolicy) validate(header *tar.Header) (string, error) {
	policy.entries++
	if policy.entries > maxArchiveEntries {
		return "", fmt.Errorf("archive: exceeds %d entries", maxArchiveEntries)
	}
	raw := header.Name
	if header.Typeflag == tar.TypeDir {
		raw = strings.TrimSuffix(raw, "/")
	}
	clean := path.Clean(raw)
	if raw == "" || clean == "." || clean != raw || path.IsAbs(raw) || clean == ".." ||
		strings.HasPrefix(clean, "../") || strings.ContainsAny(raw, "\\\x00") {
		return "", fmt.Errorf("archive: invalid path %s", header.Name)
	}
	segments := strings.Split(clean, "/")
	if len(segments) > maxArchivePathDepth {
		return "", fmt.Errorf("archive: path exceeds %d segments", maxArchivePathDepth)
	}
	for _, segment := range segments {
		if err := validatePortableArchiveSegment(segment); err != nil {
			return "", fmt.Errorf("archive: invalid path %s: %w", header.Name, err)
		}
	}
	if err := policy.registerPath(clean, header.Typeflag == tar.TypeDir); err != nil {
		return "", err
	}
	if header.Size < 0 || header.Size > maxArchiveFileBytes {
		return "", fmt.Errorf("archive: unreasonable file size for %s", clean)
	}
	if header.Size > maxArchiveExpandedBytes-policy.expandedBytes {
		return "", fmt.Errorf("archive: expanded contents exceed %d bytes", maxArchiveExpandedBytes)
	}
	policy.expandedBytes += header.Size
	switch header.Typeflag {
	case tar.TypeDir:
		if header.Size != 0 {
			return "", fmt.Errorf("archive: directory %s has a non-zero size", clean)
		}
	case tar.TypeReg:
		// Accepted after path, duplicate, and expansion checks above.
	case tar.TypeSymlink, tar.TypeLink:
		return "", fmt.Errorf("archive: symlinks/hardlinks are not allowed")
	default:
		return "", fmt.Errorf("archive: special files are not allowed")
	}
	return clean, nil
}

func (policy *archivePolicy) registerPath(name string, directory bool) error {
	parts := strings.Split(name, "/")
	current := ""
	for index, part := range parts {
		if current == "" {
			current = part
		} else {
			current += "/" + part
		}
		key := strings.ToLower(current)
		existing, found := policy.paths[key]
		if found && existing.name != current {
			return fmt.Errorf("archive: duplicate or case-colliding entries %q and %q", existing.name, current)
		}
		leaf := index == len(parts)-1
		if found && !existing.directory && (!leaf || directory) {
			return fmt.Errorf("archive: file/directory conflict at %q", current)
		}
		if !leaf {
			if !found {
				policy.paths[key] = archivePath{name: current, directory: true}
			}
			continue
		}
		if found && existing.explicit {
			return fmt.Errorf("archive: duplicate or case-colliding entries %q and %q", existing.name, current)
		}
		if found && existing.directory && !directory {
			return fmt.Errorf("archive: file/directory conflict at %q", current)
		}
		policy.paths[key] = archivePath{name: current, directory: directory, explicit: true}
	}
	return nil
}

func validatePortableArchiveSegment(segment string) error {
	if segment == "" || len(segment) > 255 || strings.HasSuffix(segment, " ") || strings.HasSuffix(segment, ".") {
		return fmt.Errorf("non-portable path segment")
	}
	for _, char := range segment {
		if char < 0x20 || char > 0x7e || strings.ContainsRune(`\/:*?"<>|`, char) {
			return fmt.Errorf("non-portable path segment")
		}
	}
	base := strings.ToUpper(strings.SplitN(segment, ".", 2)[0])
	if _, reserved := windowsReservedArchiveNames[base]; reserved {
		return fmt.Errorf("Windows reserved path segment")
	}
	return nil
}

func skipName(base string) bool {
	b := strings.ToLower(base)
	if _, ok := skipRootNames[b]; ok {
		return true
	}
	if strings.HasSuffix(b, ".txt") || strings.HasSuffix(b, ".md") {
		return true
	}
	return false
}

// ExtractRootExecutable implements ports.ArchiveExtractor.
func (TarGzExtractor) ExtractRootExecutable(ctx context.Context, r io.Reader) (name string, data []byte, err error) {
	zr, err := gzip.NewReader(r)
	if err != nil {
		return "", nil, domain.NewError(domain.ExitAmbiguous, "archive: not a valid gzip")
	}
	defer zr.Close()

	tr := tar.NewReader(zr)
	policy := newArchivePolicy()
	var candidates []struct {
		name string
		data []byte
	}

	for {
		select {
		case <-ctx.Done():
			return "", nil, domain.NewError(domain.ExitNetwork, ctx.Err().Error())
		default:
		}
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", nil, domain.NewError(domain.ExitAmbiguous, "archive: corrupt tar: "+err.Error())
		}
		clean, policyErr := policy.validate(hdr)
		if policyErr != nil {
			return "", nil, domain.NewError(domain.ExitAmbiguous, policyErr.Error())
		}
		if hdr.Typeflag == tar.TypeDir {
			continue
		}
		if strings.Contains(clean, "/") {
			if _, skipErr := io.CopyN(io.Discard, tr, hdr.Size); skipErr != nil {
				return "", nil, domain.NewError(domain.ExitAmbiguous, "archive: skip nested: "+skipErr.Error())
			}
			continue
		}
		base := path.Base(clean)
		if skipName(base) {
			if _, skipErr := io.CopyN(io.Discard, tr, hdr.Size); skipErr != nil {
				return "", nil, domain.NewError(domain.ExitAmbiguous, "archive: skip file: "+skipErr.Error())
			}
			continue
		}
		buf := make([]byte, hdr.Size)
		if _, err := io.ReadFull(tr, buf); err != nil {
			return "", nil, domain.NewError(domain.ExitAmbiguous, "archive: read "+base+": "+err.Error())
		}
		candidates = append(candidates, struct {
			name string
			data []byte
		}{name: base, data: buf})
	}

	if len(candidates) == 0 {
		return "", nil, domain.NewError(domain.ExitAmbiguous, "archive: no plugin binary in tarball root (expected one file, e.g. from GoReleaser)")
	}
	if len(candidates) > 1 {
		var names []string
		for _, c := range candidates {
			names = append(names, c.name)
		}
		return "", nil, domain.NewError(domain.ExitAmbiguous, fmt.Sprintf("archive: multiple root files after filtering: %v", names))
	}
	out := make([]byte, len(candidates[0].data))
	copy(out, candidates[0].data)
	return candidates[0].name, out, nil
}
