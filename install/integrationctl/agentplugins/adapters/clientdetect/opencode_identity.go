package clientdetect

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// Resolve every path component, including directory symlinks, with a hard link
// budget. The ordered locator/link pairs are hashed, not exposed publicly.
func nativeLocator(requested string) (string, []string, error) {
	volume := filepath.VolumeName(requested)
	current := volume + string(os.PathSeparator)
	pending := requested[len(volume):]
	var chain []string
	for range 4096 {
		pending = strings.TrimLeftFunc(pending, func(r rune) bool {
			return r == rune(os.PathSeparator) || r == '/'
		})
		if pending == "" {
			return current, chain, nil
		}
		end := 0
		for end < len(pending) && !os.IsPathSeparator(pending[end]) {
			end++
		}
		part, remainder := pending[:end], pending[end:]
		pending = remainder
		switch part {
		case ".":
			continue
		case "..":
			current = filepath.Dir(current)
			continue
		}
		current = filepath.Join(current, part) // one ordinary component only
		info, err := os.Lstat(current)
		if err != nil {
			return "", nil, err
		}
		if info.Mode()&os.ModeSymlink == 0 {
			if remainder != "" && !info.IsDir() {
				return "", nil, ErrUnverifiedProbeTarget
			}
			continue
		}
		if len(chain)/2 >= 32 {
			return "", nil, ErrUnverifiedProbeTarget
		}
		link, err := os.Readlink(current)
		if err != nil {
			return "", nil, err
		}
		chain = append(chain, current, link)
		current, pending, err = nativeLinkRemainder(current, link, remainder)
		if err != nil {
			return "", nil, err
		}
	}
	return "", nil, ErrUnverifiedProbeTarget
}

func nativeLinkRemainder(locator, link, remainder string) (string, string, error) {
	base := filepath.Dir(locator)
	if filepath.IsAbs(link) {
		volume := filepath.VolumeName(link)
		base = volume + string(os.PathSeparator)
		link = link[len(volume):]
	} else if filepath.VolumeName(link) != "" || link != "" && os.IsPathSeparator(link[0]) {
		// Drive-relative/root-relative Windows links have no pinned base.
		return "", "", ErrUnverifiedProbeTarget
	}
	// Like EvalSymlinks, splice the raw link ahead of the raw remainder.
	// Cleaning here would resolve dirlink/.. against the wrong directory.
	return base, link + remainder, nil
}

func openCodeTargetIdentity(ctx context.Context, target ProbeTarget) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	resolved, chain, err := nativeLocator(target.Executable)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(resolved)
	if err != nil {
		return "", err
	}
	if !info.Mode().IsRegular() {
		return "", ErrUnverifiedProbeTarget
	}
	file, err := os.Open(resolved)
	if err != nil {
		return "", err
	}
	defer func() { _ = file.Close() }()
	info, err = file.Stat()
	if err != nil {
		return "", err
	}
	if !info.Mode().IsRegular() {
		return "", ErrUnverifiedProbeTarget
	}
	var header [4]byte
	if _, err := io.ReadFull(file, header[:]); err != nil {
		return "", ErrUnverifiedProbeTarget
	}
	// ELF, PE, and Mach-O (including universal) are the only initial formats.
	magic := hex.EncodeToString(header[:])
	native := magic == "7f454c46" || header[0] == 'M' && header[1] == 'Z'
	switch magic {
	case "feedface", "cefaedfe", "feedfacf", "cffaedfe", "cafebabe", "bebafeca", "cafebabf", "bfbafeca":
		native = true
	}
	if !native {
		return "", ErrUnverifiedProbeTarget
	}
	digest := sha256.New()
	_, _ = digest.Write(header[:])
	if _, err := io.Copy(digest, probeContextReader{ctx: ctx, reader: file}); err != nil {
		return "", err
	}
	authority := []any{target.Executable, resolved, chain, hex.EncodeToString(digest.Sum(nil)), target.Environment}
	body, err := json.Marshal(authority)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:]), nil
}

type probeContextReader struct {
	ctx    context.Context
	reader io.Reader
}

func (r probeContextReader) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.reader.Read(p)
}
