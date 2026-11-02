package pathpolicy

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"unicode/utf16"
	"unicode/utf8"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
	"golang.org/x/text/unicode/norm"
)

const maxLeafIDBytes = 64

var portableLeafID = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)

var windowsReservedLeafIDs = map[string]struct{}{
	"CON": {}, "PRN": {}, "AUX": {}, "NUL": {}, "CLOCK$": {},
	"COM1": {}, "COM2": {}, "COM3": {}, "COM4": {}, "COM5": {}, "COM6": {}, "COM7": {}, "COM8": {}, "COM9": {},
	"LPT1": {}, "LPT2": {}, "LPT3": {}, "LPT4": {}, "LPT5": {}, "LPT6": {}, "LPT7": {}, "LPT8": {}, "LPT9": {},
	// Win32 also recognizes ISO-8859-1 superscript 1, 2 and 3 in device names.
	// Keep these in the same rule used by ZIP/TAR, Git and local snapshots.
	"COM¹": {}, "COM²": {}, "COM³": {}, "LPT¹": {}, "LPT²": {}, "LPT³": {},
}

// ValidatePortablePathSegment validates one package-tree path component while
// still allowing the Unicode names permitted by Agent Plugins packages.
func ValidatePortablePathSegment(value string) error {
	if value == "" || value == "." || value == ".." || !utf8.ValidString(value) || !norm.NFC.IsNormalString(value) {
		return fmt.Errorf("invalid path segment")
	}
	if len(utf16.Encode([]rune(value))) > 255 {
		return fmt.Errorf("path segment exceeds 255 UTF-16 code units")
	}
	if strings.ContainsAny(value, `\\/:*?"<>|`) || strings.HasSuffix(value, " ") || strings.HasSuffix(value, ".") {
		return fmt.Errorf("path segment is not portable")
	}
	for _, char := range value {
		if char < 0x20 {
			return fmt.Errorf("path segment contains a control character")
		}
	}
	base := strings.ToUpper(strings.TrimRight(strings.SplitN(value, ".", 2)[0], " ."))
	if _, reserved := windowsReservedLeafIDs[base]; reserved {
		return fmt.Errorf("Windows reserved path segment")
	}
	return nil
}

// ValidateLeafID validates an identifier before it is used as one physical
// filesystem path component. Product-specific schemas may impose stricter
// semantic rules in addition to this portable path-safety invariant.
func ValidateLeafID(value string) error {
	// A validator must not silently canonicalize a different path identity.
	// A caller using the original string would otherwise bypass the ASCII leaf rule.
	if value != strings.TrimSpace(value) {
		return fmt.Errorf("identifier must not have leading or trailing whitespace")
	}
	if value == "" {
		return fmt.Errorf("identifier is required")
	}
	if len(value) > maxLeafIDBytes {
		return fmt.Errorf("identifier exceeds %d bytes", maxLeafIDBytes)
	}
	if !utf8.ValidString(value) || strings.ContainsRune(value, '\x00') {
		return fmt.Errorf("identifier is not valid UTF-8 text")
	}
	if value == "." || value == ".." || filepath.IsAbs(value) || filepath.VolumeName(value) != "" {
		return fmt.Errorf("identifier must be a relative leaf name")
	}
	if strings.ContainsAny(value, `/\\`) || !portableLeafID.MatchString(value) {
		return fmt.Errorf("identifier must contain only ASCII letters, digits, dot, underscore, or hyphen")
	}
	if strings.HasSuffix(value, ".") || strings.Contains(value, "..") {
		return fmt.Errorf("identifier contains a non-portable dot segment")
	}
	base := strings.ToUpper(strings.SplitN(value, ".", 2)[0])
	if _, reserved := windowsReservedLeafIDs[base]; reserved {
		return fmt.Errorf("identifier is reserved on Windows")
	}
	return nil
}

// RequireContainedChild rejects a destructive target unless it is a strict
// lexical child of base and no existing component at or below base is a
// symlink. Call it immediately before remove/replace operations.
func RequireContainedChild(base, candidate string) error {
	if strings.TrimSpace(base) == "" || strings.TrimSpace(candidate) == "" {
		return fmt.Errorf("managed base and candidate path are required")
	}
	absBase, err := filepath.Abs(base)
	if err != nil {
		return fmt.Errorf("resolve managed base: %w", err)
	}
	absCandidate, err := filepath.Abs(candidate)
	if err != nil {
		return fmt.Errorf("resolve managed candidate: %w", err)
	}
	absBase = filepath.Clean(absBase)
	absCandidate = filepath.Clean(absCandidate)
	rel, err := filepath.Rel(absBase, absCandidate)
	if err != nil {
		return fmt.Errorf("compare managed paths: %w", err)
	}
	if rel == "." || rel == ".." || filepath.IsAbs(rel) || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return fmt.Errorf("path %q is not a strict child of managed base %q", absCandidate, absBase)
	}
	current := absBase
	parts := strings.Split(rel, string(filepath.Separator))
	for index := -1; index < len(parts); index++ {
		if index >= 0 {
			current = filepath.Join(current, parts[index])
		}
		info, statErr := os.Lstat(current)
		if os.IsNotExist(statErr) {
			continue
		}
		if statErr != nil {
			return fmt.Errorf("inspect managed path %q: %w", current, statErr)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("managed path %q contains a symlink component", current)
		}
		if index < len(parts)-1 && !info.IsDir() {
			return fmt.Errorf("managed path ancestor %q is not a directory", current)
		}
	}
	return nil
}

// RequireExactPath verifies that persisted metadata still points at the one
// deterministic managed path computed by the adapter.
func RequireExactPath(expected, candidate string) error {
	if strings.TrimSpace(expected) == "" || strings.TrimSpace(candidate) == "" {
		return fmt.Errorf("expected and candidate path are required")
	}
	absExpected, err := filepath.Abs(expected)
	if err != nil {
		return fmt.Errorf("resolve expected managed path: %w", err)
	}
	absCandidate, err := filepath.Abs(candidate)
	if err != nil {
		return fmt.Errorf("resolve candidate managed path: %w", err)
	}
	if filepath.Clean(absExpected) != filepath.Clean(absCandidate) {
		return fmt.Errorf("persisted path %q does not match managed path %q", filepath.Clean(absCandidate), filepath.Clean(absExpected))
	}
	return RequireContainedChild(filepath.Dir(absExpected), absCandidate)
}

func UserHome(explicit string) string {
	if strings.TrimSpace(explicit) != "" {
		return explicit
	}
	home, _ := os.UserHomeDir()
	return home
}

func ProjectRoot(workspaceRoot, projectRoot string) string {
	if root := strings.TrimSpace(workspaceRoot); root != "" {
		return filepath.Clean(root)
	}
	if root := strings.TrimSpace(projectRoot); root != "" {
		return filepath.Clean(root)
	}
	cwd, _ := os.Getwd()
	return filepath.Clean(cwd)
}

func EffectiveGitRoot(workspaceRoot, projectRoot string) string {
	fallback := ProjectRoot(workspaceRoot, projectRoot)
	root := filepath.Clean(fallback)
	for {
		if root == "." || root == string(filepath.Separator) || strings.TrimSpace(root) == "" {
			return fallback
		}
		if FileExists(filepath.Join(root, ".git")) {
			return root
		}
		parent := filepath.Dir(root)
		if parent == root {
			return fallback
		}
		root = parent
	}
}

func PreferredExistingPath(candidates ...string) string {
	for _, path := range candidates {
		if FileExists(path) {
			return path
		}
	}
	for _, path := range candidates {
		if strings.TrimSpace(path) != "" {
			return path
		}
	}
	return ""
}

func FileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func NormalizeScope(scope string) string {
	if strings.EqualFold(strings.TrimSpace(scope), "project") {
		return "project"
	}
	return "user"
}

func WorkspaceRootFromInspect(in ports.InspectInput) string {
	if in.Record != nil {
		return WorkspaceRootFromRecord(*in.Record)
	}
	return ""
}

func WorkspaceRootFromApply(in ports.ApplyInput) string {
	if in.Record != nil {
		return WorkspaceRootFromRecord(*in.Record)
	}
	return ""
}

func WorkspaceRootFromRecord(record domain.InstallationRecord) string {
	if NormalizeScope(record.Policy.Scope) == "project" {
		return strings.TrimSpace(record.WorkspaceRoot)
	}
	return ""
}

func ProtectionForScope(scope string) domain.ProtectionClass {
	if NormalizeScope(scope) == "project" {
		return domain.ProtectionWorkspace
	}
	return domain.ProtectionUserMutable
}
