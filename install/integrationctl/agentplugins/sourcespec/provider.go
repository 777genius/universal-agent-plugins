package sourcespec

import (
	"errors"
	"net/url"
	"strings"
)

func parseProvider(u *url.URL) (Git, bool, error) {
	host := strings.ToLower(u.Hostname())
	if encodedSeparator(u.RawPath) && (host == "github.com" || host == "gitlab.com" || isAzureHost(host) || strings.Contains(u.Path, "/-/tree/")) {
		return Git{}, true, errors.New("encoded browser path separators are ambiguous; use the clone URL with an explicit ref and package path")
	}
	if isAzureHost(host) {
		g, err := parseAzure(u)
		return g, true, err
	}
	if host == "github.com" {
		g, err := parseGitHubBrowser(u)
		return g, true, err
	}
	if strings.Contains(u.Path, "/-/tree/") || host == "gitlab.com" {
		g, err := parseGitLabBrowser(u)
		return g, true, err
	}
	return Git{}, false, nil
}

func parseGitHubBrowser(u *url.URL) (Git, error) {
	segments := strings.Split(strings.Trim(u.Path, "/"), "/")
	if len(segments) < 2 {
		return Git{}, errors.New("GitHub source requires owner/repo")
	}
	if u.RawQuery != "" || u.ForceQuery {
		return Git{}, errors.New("GitHub source cannot contain query parameters")
	}
	base := *u
	base.Path, base.RawPath, base.RawQuery = "/"+strings.Join(segments[:2], "/"), "", ""
	endpoint, repo, err := NormalizeGitURL(base.String())
	if err != nil {
		return Git{}, err
	}
	g := Git{URL: endpoint, Repository: repo}
	if len(segments) == 2 {
		return g, nil
	}
	if len(segments) < 4 || segments[2] != "tree" {
		return Git{}, errors.New("choose a repository or tree package root URL")
	}
	g.Browser = &BrowserTree{Provider: "github", Suffix: strings.Join(segments[3:], "/")}
	return g, nil
}

func parseGitLabBrowser(u *url.URL) (Git, error) {
	if u.RawQuery != "" || u.ForceQuery {
		return Git{}, errors.New("GitLab source cannot contain query parameters")
	}
	marker := "/-/tree/"
	base := *u
	var browser *BrowserTree
	if index := strings.Index(u.Path, marker); index >= 0 {
		base.Path = u.Path[:index]
		browser = &BrowserTree{Provider: "gitlab", Suffix: u.Path[index+len(marker):]}
	} else if len(strings.Split(strings.Trim(u.Path, "/"), "/")) < 2 || strings.Contains(u.Path, "/-/") {
		return Git{}, errors.New("choose a GitLab repository or tree package root URL")
	}
	base.Path = strings.TrimSuffix(base.Path, "/") + ".git"
	base.RawPath, base.RawQuery = "", ""
	endpoint, repo, err := NormalizeGitURL(base.String())
	return Git{URL: endpoint, Repository: repo, Browser: browser}, err
}

func parseAzure(u *url.URL) (Git, error) {
	if err := validateAzurePath(u.Path); err != nil {
		return Git{}, err
	}
	q, err := azureQuery(u.RawQuery)
	if err != nil {
		return Git{}, err
	}
	base := *u
	base.RawQuery = ""
	base.ForceQuery = false
	base.Fragment = ""
	endpoint, repo, err := NormalizeGitURL(base.String())
	if err != nil {
		return Git{}, err
	}
	g := Git{URL: endpoint, Repository: repo}
	g.Path, err = NormalizeSubpath(strings.TrimPrefix(q.Get("path"), "/"))
	if err != nil {
		return Git{}, err
	}
	return azureVersion(g, q.Get("version"))
}

func validateAzurePath(value string) error {
	segments := strings.Split(strings.Trim(value, "/"), "/")
	index := -1
	for i, s := range segments {
		if s == "_git" {
			if index >= 0 {
				return errors.New("invalid Azure repository URL")
			}
			index = i
		}
	}
	if index < 1 || index+2 != len(segments) {
		return errors.New("azure source requires an _git repository URL")
	}
	return nil
}

func azureQuery(raw string) (url.Values, error) {
	q, err := url.ParseQuery(raw)
	if err != nil {
		return nil, errors.New("invalid Azure source query")
	}
	for key, values := range q {
		if len(values) != 1 || (key != "path" && key != "version" && key != "_a") {
			return nil, errors.New("unsupported Azure source query")
		}
	}
	if q.Get("_a") != "" && q.Get("_a") != "contents" {
		return nil, errors.New("choose Azure repository contents")
	}
	return q, nil
}

func azureVersion(g Git, version string) (Git, error) {
	if version != "" {
		if len(version) < 3 {
			return Git{}, errors.New("invalid Azure source version")
		}
		switch version[:2] {
		case "GB":
			g.Ref = "refs/heads/" + version[2:]
		case "GT":
			g.Ref = "refs/tags/" + version[2:]
		case "GC":
			if !ValidCommit(version[2:]) {
				return Git{}, errors.New("azure commit requires a full lowercase SHA")
			}
			g.Commit = version[2:]
		default:
			return Git{}, errors.New("unsupported Azure source version")
		}
		if g.Ref != "" {
			if err := ValidateRef(g.Ref); err != nil {
				return Git{}, err
			}
		}
	}
	return g, nil
}

func encodedSeparator(raw string) bool {
	value := strings.ToLower(raw)
	return strings.Contains(value, "%2f") || strings.Contains(value, "%5c")
}
