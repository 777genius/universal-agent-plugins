package sourcespec

import (
	"errors"
	"net/url"
	"strings"
	"unicode"
)

// NormalizeGitURL accepts only native HTTPS/SSH transports, never local paths,
// remote helpers, passwords or query aliases. SCP paths retain relative-path
// semantics instead of being changed into absolute ssh:// paths.
func NormalizeGitURL(value string) (string, string, error) {
	if !safeText(value) {
		return "", "", errors.New("unsafe Git endpoint")
	}
	if !strings.Contains(value, "://") {
		return normalizeSCP(value)
	}

	u, err := url.Parse(value)
	if err != nil || (u.Scheme != "https" && u.Scheme != "ssh") {
		return "", "", errors.New("Git endpoint must use HTTPS or SSH")
	}
	u, err = normalizedURL(value, u.Scheme)
	if err != nil || u.RawQuery != "" || u.ForceQuery || u.Fragment != "" {
		return "", "", errors.New("Git endpoint cannot contain query or fragment")
	}
	if err := normalizeGitUser(u); err != nil {
		return "", "", err
	}
	repository := ""
	if u.Scheme == "https" && u.Host == "github.com" {
		if encodedSeparator(u.RawPath) {
			return "", "", errors.New("encoded repository separators require an unambiguous clone URL")
		}
		repository = strings.TrimSuffix(strings.TrimPrefix(u.Path, "/"), ".git")
		if !repositoryPattern.MatchString(repository) {
			return "", "", errors.New("GitHub repository must be owner/repo")
		}
		repository = strings.ToLower(repository)
		u.Path = "/" + repository + ".git"
	}
	return u.String(), repository, nil
}

func normalizeGitUser(u *url.URL) error {
	if u.User != nil {
		_, password := u.User.Password()
		if password || u.User.Username() == "" || strings.ContainsAny(u.User.Username(), "@:/ ") || strings.ContainsFunc(u.User.Username(), unicode.IsControl) {
			return errors.New("Git endpoint cannot contain credentials")
		}
		if u.Scheme == "https" {
			if !isAzureHost(u.Hostname()) {
				return errors.New("HTTPS Git endpoint cannot contain user information")
			}
			u.User = nil
		}
	}
	return nil
}

func normalizeSCP(value string) (string, string, error) {
	left, right, ok := strings.Cut(value, ":")
	if !ok || right == "" || strings.HasPrefix(right, "-") || strings.HasPrefix(left, "-") || strings.ContainsAny(left, " /?#") || strings.ContainsAny(right, "?#") || (!strings.Contains(left, "@") && !strings.Contains(left, ".")) {
		return "", "", errors.New("Git endpoint must use HTTPS, ssh:// or user@host:path")
	}
	user, host, hasUser := strings.Cut(left, "@")
	if hasUser {
		if user == "" || host == "" || strings.Contains(host, "@") {
			return "", "", errors.New("invalid SSH endpoint")
		}
	} else {
		host = user
		user = ""
	}
	if strings.ContainsAny(host, "[]: ") || strings.ContainsAny(user, ": ") || strings.HasPrefix(host, "-") {
		return "", "", errors.New("invalid SSH endpoint")
	}
	if err := validateSCPPath(right); err != nil {
		return "", "", err
	}
	prefix := strings.ToLower(host)
	if hasUser {
		prefix = user + "@" + prefix
	}
	return prefix + ":" + right, "", nil
}

func validateSCPPath(right string) error {
	for index, segment := range strings.Split(right, "/") {
		if index == 0 && segment == "" && strings.HasPrefix(right, "/") {
			continue
		}
		if segment == "." || segment == ".." || segment == "" {
			return errors.New("unsafe SSH repository path")
		}
	}
	return nil
}

func isAzureHost(host string) bool {
	return host == "dev.azure.com" || strings.HasSuffix(host, ".visualstudio.com")
}

func parseGit(value string) (Git, error) {
	base, fragment, hasFragment := strings.Cut(value, "#")
	if strings.Contains(fragment, "#") {
		return Git{}, errors.New("invalid Git selector")
	}
	packagePath := ""
	if before, after, ok := strings.Cut(fragment, "//"); ok {
		fragment, packagePath = before, after
	}
	if before, after, ok := strings.Cut(base, ".git//"); ok {
		if packagePath != "" {
			return Git{}, errors.New("duplicate package path")
		}
		base, packagePath = before+".git", after
	}
	var err error
	packagePath, err = NormalizeSubpath(packagePath)
	if err != nil {
		return Git{}, err
	}
	endpoint, repo, err := NormalizeGitURL(strings.TrimPrefix(base, "git+"))
	if err != nil {
		return Git{}, err
	}
	g := Git{URL: endpoint, Repository: repo, Path: packagePath}
	if hasFragment {
		fragment = strings.TrimPrefix(fragment, "ref=")
		if err := ValidateRef(fragment); err != nil {
			return Git{}, err
		}
		if ValidCommit(fragment) {
			g.Commit = fragment
		} else {
			g.Ref = fragment
		}
	}
	return g, nil
}

func parseShorthand(value string) (Git, error) {
	value = strings.TrimPrefix(value, "github:")
	base, packagePath, _ := strings.Cut(value, "//")
	repository, selector := base, ""
	if index := strings.LastIndex(base, "@"); index >= 0 {
		repository, selector = base[:index], base[index+1:]
		if selector == "" {
			return Git{}, errors.New("empty Git selector")
		}
	}
	if !repositoryPattern.MatchString(repository) {
		return Git{}, errors.New("GitHub source requires owner/repo")
	}
	g, err := parseGit("https://github.com/" + repository + ".git")
	if err != nil {
		return Git{}, err
	}
	g.Path, err = NormalizeSubpath(packagePath)
	if err != nil {
		return Git{}, err
	}
	if selector != "" {
		if err := ValidateRef(selector); err != nil {
			return Git{}, err
		}
		if ValidCommit(selector) {
			g.Commit = selector
		} else {
			g.Ref = selector
		}
	}
	return g, nil
}
