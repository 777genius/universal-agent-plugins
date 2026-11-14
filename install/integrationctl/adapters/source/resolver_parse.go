package source

import (
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"
)

var githubRepositoryPattern = regexp.MustCompile(`^[A-Za-z0-9](?:[A-Za-z0-9-]{0,37}[A-Za-z0-9])?/[A-Za-z0-9._-]{1,100}$`)

func resolveLocal(raw string) (string, bool) {
	path := filepath.Clean(raw)
	if strings.HasPrefix(raw, ".") || strings.HasPrefix(raw, "/") {
		abs, _ := filepath.Abs(path)
		if info, err := os.Stat(abs); err == nil && info.IsDir() {
			return abs, true
		}
	}
	abs, _ := filepath.Abs(path)
	if info, err := os.Stat(abs); err == nil && info.IsDir() {
		return abs, true
	}
	return "", false
}

func parseGitHubRef(raw string) (ownerRepo, gitRef, subdir string, ok bool) {
	value := strings.TrimPrefix(raw, "github:")
	parts := strings.SplitN(value, "//", 2)
	ownerRepo = strings.TrimSpace(parts[0])
	if ownerRepo == "" {
		return "", "", "", false
	}
	if idx := strings.LastIndex(ownerRepo, "@"); idx > 0 {
		gitRef = strings.TrimSpace(ownerRepo[idx+1:])
		ownerRepo = strings.TrimSpace(ownerRepo[:idx])
	}
	if ownerRepo == "" || !githubRepositoryPattern.MatchString(ownerRepo) || strings.HasSuffix(ownerRepo, "/.") || strings.HasSuffix(ownerRepo, "/..") {
		return "", "", "", false
	}
	if !validGitRefInput(gitRef) {
		return "", "", "", false
	}
	if len(parts) == 2 {
		var err error
		subdir, err = normalizePackageSubdir(parts[1])
		if err != nil {
			return "", "", "", false
		}
	}
	return ownerRepo, gitRef, subdir, true
}

func normalizePackageSubdir(raw string) (string, error) {
	value := strings.TrimSpace(raw)
	if value == "" || !utf8.ValidString(value) || !norm.NFC.IsNormalString(value) {
		return "", errUnsafePackageSubdir
	}
	if strings.ContainsAny(value, "\\\x00") || strings.HasPrefix(value, "/") || hasWindowsVolumePrefix(value) {
		return "", errUnsafePackageSubdir
	}
	if len(value) > maxPackageSubdirBytes {
		return "", errUnsafePackageSubdir
	}
	parts := strings.Split(value, "/")
	if len(parts) > maxTreeDepth {
		return "", errUnsafePackageSubdir
	}
	for _, part := range parts {
		if err := validatePortablePathSegment(part); err != nil {
			return "", errUnsafePackageSubdir
		}
	}
	cleaned := path.Clean(value)
	if cleaned != value || cleaned == "." || strings.HasPrefix(cleaned, "../") {
		return "", errUnsafePackageSubdir
	}
	return cleaned, nil
}

func parseGitURLRef(raw string) (repoURL, gitRef string, ok bool) {
	repoURL = strings.TrimSpace(raw)
	if repoURL == "" || strings.HasPrefix(repoURL, "-") || strings.ContainsAny(repoURL, "\x00\r\n\t") {
		return "", "", false
	}
	if idx := strings.LastIndex(repoURL, "#"); idx >= 0 {
		gitRef = normalizeGitRef(repoURL[idx+1:])
		repoURL = strings.TrimSpace(repoURL[:idx])
	}
	if !validGitRefInput(gitRef) || !isGitURL(repoURL) {
		return "", "", false
	}
	return repoURL, gitRef, true
}

func validGitRefInput(value string) bool {
	return len(value) <= 1024 && !strings.HasPrefix(value, "-") && !strings.ContainsAny(value, "\x00\r\n\t")
}

func normalizeGitRef(raw string) string {
	raw = strings.TrimSpace(raw)
	raw = strings.TrimPrefix(raw, "ref=")
	return strings.TrimSpace(raw)
}

func isGitURL(raw string) bool {
	if strings.HasPrefix(raw, "git@") {
		colon := strings.IndexByte(raw, ':')
		return colon > len("git@") && colon < len(raw)-1 && strings.HasSuffix(raw[colon+1:], ".git") && !strings.ContainsAny(raw, " @\x00\r\n\t")
	}
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" || u.RawQuery != "" || u.Fragment != "" || !strings.HasSuffix(u.Path, ".git") {
		return false
	}
	switch u.Scheme {
	case "https":
		return u.User == nil
	case "ssh":
		if u.User == nil {
			return true
		}
		_, hasPassword := u.User.Password()
		return !hasPassword && u.User.Username() != ""
	default:
		return false
	}
}
