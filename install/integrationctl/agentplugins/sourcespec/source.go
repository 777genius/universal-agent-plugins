// Package sourcespec defines the offline grammar and stable identity of direct
// package sources. It never contacts a host or reads package content.
package sourcespec

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/url"
	"path"
	"regexp"
	"strings"
	"unicode"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/pathpolicy"
)

type Kind string

const (
	KindGit          Kind = "git"
	KindArchive      Kind = "archive"
	KindHTTPSAuto    Kind = "https-auto"
	KindLocalArchive Kind = "local-archive"
)

// Git describes intent. Browser is transient and must be resolved before any
// coordinates are persisted. Repository identifies canonical HTTPS GitHub only.
type Git struct {
	URL, Repository, Path, Ref, Commit string
	Browser                            *BrowserTree
}

type BrowserTree struct{ Provider, Suffix string }

// Archive.URL is a canonical HTTPS endpoint or, for an explicit local input,
// the file path. Local archives are never portable saved requirements.
type Archive struct{ URL, Path, SHA256 string }

type Spec struct {
	Kind      Kind
	Git       *Git
	Archive   *Archive
	Requested string
}

var (
	commitPattern     = regexp.MustCompile(`^[0-9a-f]{40}$`)
	digestPattern     = regexp.MustCompile(`^[0-9a-f]{64}$`)
	repositoryPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9-]*/[A-Za-z0-9][A-Za-z0-9._-]*$`)
)

func ValidCommit(value string) bool { return commitPattern.MatchString(value) }
func ValidSHA256(value string) bool { return digestPattern.MatchString(value) }

func safeText(value string) bool {
	return value != "" && len(value) <= 8192 && strings.TrimSpace(value) == value &&
		!strings.ContainsAny(value, `\`) && !strings.ContainsFunc(value, unicode.IsControl)
}

func NormalizeSubpath(value string) (string, error) {
	if value == "" {
		return "", nil
	}
	if len(value) > 1024 || path.IsAbs(value) || path.Clean(value) != value || len(strings.Split(value, "/")) > 64 {
		return "", errors.New("package path must be a normalized portable relative directory")
	}
	for _, segment := range strings.Split(value, "/") {
		if err := pathpolicy.ValidatePortablePathSegment(segment); err != nil {
			return "", errors.New("package path must be a normalized portable relative directory")
		}
	}
	return value, nil
}

func ValidateRef(value string) error {
	if value == "" || len(value) > 1024 || !safeText(value) || strings.HasPrefix(value, "-") || strings.ContainsAny(value, " ~^:?*[\\") || strings.Contains(value, "..") || strings.Contains(value, "@{") || strings.Contains(value, "//") || strings.HasSuffix(value, ".") || strings.HasSuffix(value, "/") || strings.HasPrefix(value, "/") {
		return errors.New("Git selector must be an exact branch, tag or full lowercase commit SHA")
	}
	for _, segment := range strings.Split(value, "/") {
		if strings.HasPrefix(segment, ".") || strings.HasSuffix(segment, ".lock") {
			return errors.New("unsafe Git ref")
		}
	}
	return nil
}

// Key hashes a length-unambiguous canonical tuple; revision is intentionally
// excluded so updates retain ownership. Historical GitHub keys stay unchanged.
func Key(kind Kind, endpoint, packagePath string) string {
	body, _ := json.Marshal([3]string{string(kind), endpoint, packagePath})
	sum := sha256.Sum256(body)
	prefix := "git-url-v1:"
	if kind == KindArchive {
		prefix = "archive-url-v1:"
	}
	return prefix + hex.EncodeToString(sum[:])
}

func HasArchiveSuffix(value string) bool {
	value = strings.ToLower(value)
	for _, suffix := range []string{".zip", ".tar", ".tar.gz", ".tgz"} {
		if strings.HasSuffix(value, suffix) {
			return true
		}
	}
	return false
}

func NormalizeArchiveURL(value string) (string, error) {
	u, err := normalizedURL(value, "https")
	if err != nil || u.User != nil || u.RawQuery != "" || u.ForceQuery || u.Fragment != "" {
		return "", errors.New("archive source requires a credential-free HTTPS URL without query or fragment")
	}
	return u.String(), nil
}

func normalizedURL(value, scheme string) (*url.URL, error) {
	if !safeText(value) {
		return nil, errors.New("unsafe source URL")
	}
	u, err := url.Parse(value)
	if err != nil || u.Scheme != scheme || u.Hostname() == "" || u.Opaque != "" || u.Host != strings.TrimSpace(u.Host) || u.Path == "" || strings.HasPrefix(u.Path, "//") || strings.ContainsAny(u.Path, "\\\x00\r\n\t") || strings.ContainsFunc(u.Path, unicode.IsControl) {
		return nil, errors.New("invalid source URL")
	}
	if u.Port() != "" {
		for _, c := range u.Port() {
			if c < '0' || c > '9' {
				return nil, errors.New("invalid source port")
			}
		}
	}
	// Re-encoding must not silently change endpoint path semantics.
	for _, segment := range strings.Split(u.Path, "/") {
		if segment == "." || segment == ".." {
			return nil, errors.New("unsafe source URL path")
		}
	}
	u.Scheme = strings.ToLower(u.Scheme)
	u.Host = strings.ToLower(u.Host)
	u.Path = strings.TrimSuffix(u.Path, "/")
	if u.Path == "" {
		return nil, errors.New("source URL requires a repository or package path")
	}
	u.RawPath = strings.TrimSuffix(u.RawPath, "/")
	return u, nil
}
