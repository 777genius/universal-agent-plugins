// Package pathcontract keeps portable path syntax separate from filesystem readiness.
package pathcontract

import (
	"fmt"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/conformance"
	"os"
	"path/filepath"
	"strings"
)

type Anchor string

const (
	Plugin Anchor = "plugin"
	Data   Anchor = "data"
)

type Path struct {
	Anchor   Anchor
	Relative string
}
type State string

const (
	Resolved    State = "resolved"
	Missing     State = "missing"
	Unavailable State = "unavailable"
	Invalid     State = "invalid"
)

type Result struct {
	Path  string
	State State
	Err   error
}

func ParseCommand(value string) (string, error) { return conformance.ParseCommandPath(value) }
func ParseCWD(value string) (Path, error) {
	anchor, relative, err := conformance.ParseCWDPath(value)
	return Path{Anchor: Anchor(anchor), Relative: relative}, err
}

// ExpandCWD expands only the raw suffix once, retaining the original anchor.
// Replacement text (including tokens occurring in root directory names) is inert.
func ExpandCWD(value, pluginRoot, dataRoot string) (Path, error) {
	parsed, err := ParseCWD(value)
	if err != nil {
		return parsed, err
	}
	parsed.Relative = strings.NewReplacer("${PLUGIN_ROOT}", pluginRoot, "${PLUGIN_DATA}", dataRoot).Replace(parsed.Relative)
	// The suffix follows a rooted separator, so repeated separators remain relative.
	parsed.Relative = strings.TrimLeft(filepath.ToSlash(parsed.Relative), "/")
	return parsed, nil
}

func contained(root, candidate string) bool {
	rel, err := filepath.Rel(root, candidate)
	return err == nil && !filepath.IsAbs(rel) && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

// Resolve observes segments in traversal order. It never cleans away an unobserved
// segment, and never returns an executable path for an incomplete observation.
func Resolve(root, relative string) Result {
	fail := func(state State, err error) Result { return Result{State: state, Err: err} }
	if strings.ContainsAny(relative, "\\\x00") || strings.HasPrefix(relative, "/") {
		return fail(Invalid, fmt.Errorf("path must remain relative to its declared root"))
	}
	canonical, err := filepath.EvalSymlinks(root)
	if err != nil {
		return fail(Unavailable, fmt.Errorf("resolve path root: %w", err))
	}
	canonical, err = filepath.Abs(canonical)
	if err != nil {
		return fail(Unavailable, err)
	}
	current := canonical
	parts := strings.Split(relative, "/")
	for i, part := range parts {
		if part == "" || part == "." {
			continue
		}
		if part == ".." {
			current = filepath.Dir(current)
			if !contained(canonical, current) {
				return fail(Invalid, fmt.Errorf("path escapes its declared root"))
			}
			continue
		}
		next := current + string(filepath.Separator) + part
		info, statErr := os.Lstat(next)
		if statErr != nil {
			if !os.IsNotExist(statErr) {
				return fail(Unavailable, statErr)
			}
			// Missing does not erase the remaining traversal. Reject a demonstrable
			// escape, but never invent a canonical target from lexical cancellation.
			probe := next
			for _, rest := range parts[i+1:] {
				if rest == ".." {
					probe = filepath.Dir(probe)
				} else if rest != "" && rest != "." {
					probe = probe + string(filepath.Separator) + rest
				}
				if !contained(canonical, probe) {
					return fail(Invalid, fmt.Errorf("unresolved path escapes its declared root"))
				}
			}
			return fail(Missing, statErr)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			target, readErr := os.Readlink(next)
			if readErr != nil {
				return fail(Unavailable, readErr)
			}
			if !filepath.IsAbs(target) {
				target = current + string(filepath.Separator) + target
			}
			next, err = filepath.EvalSymlinks(next)
			if err != nil {
				// A dangling link still cannot defer an already observable outside target.
				// Observe ancestors rather than comparing aliases lexically.
				if outsideObservedAncestor(canonical, target, 0) {
					return fail(Invalid, fmt.Errorf("symlink target escapes its declared root"))
				}

				if os.IsNotExist(err) {
					return fail(Missing, err)
				}
				return fail(Unavailable, err)
			}

		}
		if !contained(canonical, next) {
			return fail(Invalid, fmt.Errorf("path resolves outside its declared root"))
		}
		if i < len(parts)-1 && !info.IsDir() {
			targetInfo, e := os.Stat(next)
			if e != nil {
				return fail(Unavailable, e)
			}
			if !targetInfo.IsDir() {
				return fail(Unavailable, fmt.Errorf("path component is not a directory"))
			}
		}
		current = next
	}
	return Result{Path: current, State: Resolved}
}

// outsideObservedAncestor follows dangling symlink chains as well as parent
// directories. The bound prevents cycles from turning observation into a hang.
func outsideObservedAncestor(root, target string, hops int) bool {
	if hops >= 40 {
		return false
	}
	for {
		resolved, err := filepath.EvalSymlinks(target)
		if err == nil {
			return !contained(root, resolved)
		}
		info, statErr := os.Lstat(target)
		if statErr == nil && info.Mode()&os.ModeSymlink != 0 {
			link, readErr := os.Readlink(target)
			if readErr != nil {
				return false
			}
			if !filepath.IsAbs(link) {
				link = filepath.Dir(target) + string(filepath.Separator) + link
			}
			return outsideObservedAncestor(root, link, hops+1)
		}
		if !os.IsNotExist(err) {
			return false
		}
		parent := filepath.Dir(target)
		if parent == target {
			return false
		}
		target = parent
	}
}
