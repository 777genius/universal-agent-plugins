package packagedigest

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"path"
	"sort"
)

// CapturedEntry is an already bounded, privately owned input to the existing
// installer framing. Content is exact regular-file bytes, never a source path.
// Callers must independently establish link containment and capture coverage.
// This API does not acquire files or certify an atomic source revision.
type CapturedEntry struct {
	Path       string
	Kind       string // file, directory, symlink
	Executable bool
	Target     string
	Content    []byte
}

// ErrCapturedPolicy means the captured tree cannot have installer tree identity.
// It is an installer representation/content restriction, not a schema finding.
var ErrCapturedPolicy = errors.New("captured tree is outside installer digest policy")

// DigestCaptured reuses the installer path/link policy and exact v1 framing on
// safe private bytes. Its inherited lexical link-policy check is only installer
// compatibility, NOT proof of containment; the caller must prove traversal-order
// containment independently before this call. It never opens a path. Input slices remain caller-owned
// and must not change during this call. Existing snapshot behavior is unchanged.
func DigestCaptured(ctx context.Context, captured []CapturedEntry) (string, error) {
	items := append([]CapturedEntry(nil), captured...)
	sort.Slice(items, func(i, j int) bool { return items[i].Path < items[j].Path })
	seen := map[string]string{}
	known := map[string]entry{".": {rel: ".", kind: "directory"}}
	for i, c := range items {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		if i > 0 && items[i-1].Path == c.Path {
			return "", ErrCapturedPolicy
		}
		if validatePath(c.Path, 64, seen) != nil {
			return "", ErrCapturedPolicy
		}
		e := entry{rel: c.Path, kind: c.Kind, target: c.Target}
		switch c.Kind {
		case "directory":
			if c.Target != "" || len(c.Content) != 0 || c.Executable {
				return "", ErrCapturedPolicy
			}
		case "symlink":
			if validateLinkText(c.Target) != nil || len(c.Content) != 0 || c.Executable {
				return "", ErrCapturedPolicy
			}
		case "file":
			if c.Target != "" {
				return "", ErrCapturedPolicy
			}
			if len(c.Content) >= 40 && len(c.Content) <= 1024 {
				line, _, _ := bytes.Cut(c.Content, []byte{'\n'})
				if bytes.Equal(bytes.TrimSuffix(line, []byte{'\r'}), []byte("version https://git-lfs.github.com/spec/v1")) {
					return "", ErrCapturedPolicy
				}
			}
		default:
			return "", ErrCapturedPolicy
		}
		known[c.Path] = e
	}
	for _, c := range items {
		if parent, ok := known[path.Dir(c.Path)]; !ok || parent.kind != "directory" {
			return "", ErrCapturedPolicy
		}
		if c.Kind == "symlink" && resolveInternalLink(c.Path, c.Target, known) != nil {
			return "", ErrCapturedPolicy
		}
	}
	h := sha256.New()
	frame(h, []byte(digestDomain))
	for _, c := range items {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		e := entry{rel: c.Path, kind: c.Kind, target: c.Target, mode: "040000"}
		switch c.Kind {
		case "symlink":
			e.mode = "120000"
		case "file":
			e.mode = "100644"
			if c.Executable {
				e.mode = "100755"
			}
		}
		writeEntryHeader(h, e, int64(len(c.Content)))
		for b := c.Content; len(b) > 0; {
			if err := ctx.Err(); err != nil {
				return "", err
			}
			n := min(len(b), 32*1024)
			_, _ = h.Write(b[:n])
			b = b[n:]
		}
	}
	return "sha256:" + hex.EncodeToString(h.Sum(nil)), nil
}
