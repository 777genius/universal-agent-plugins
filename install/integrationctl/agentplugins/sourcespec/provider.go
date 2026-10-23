package sourcespec

import (
	"errors"
	"net/url"
	"strings"
)

func parseProvider(u *url.URL) (Git, bool, error) {
	host := strings.ToLower(u.Hostname())
	if isAzureHost(host) {
		g, err := parseAzure(u)
		return g, true, err
	}
	segments := strings.Split(strings.Trim(u.Path, "/"), "/")
	if host == "github.com" {
		if len(segments) < 2 {
			return Git{}, true, errors.New("GitHub source requires owner/repo")
		}
		base := *u
		base.Path = "/" + strings.Join(segments[:2], "/")
		base.RawQuery = ""
		base.RawPath = ""
		endpoint, repo, err := NormalizeGitURL(base.String())
		if err != nil {
			return Git{}, true, err
		}
		g := Git{URL: endpoint, Repository: repo}
		if u.RawQuery != "" || u.ForceQuery {
			return Git{}, true, errors.New("GitHub source cannot contain query parameters")
		}
		if len(segments) == 2 {
			return g, true, nil
		}
		if len(segments) < 4 || segments[2] != "tree" {
			return Git{}, true, errors.New("choose a repository or tree package root URL")
		}
		g.Browser = &BrowserTree{Provider: "github", Suffix: strings.Join(segments[3:], "/")}
		return g, true, nil
	}
	marker := "/-/tree/"
	if index := strings.Index(u.Path, marker); index >= 0 {
		base := *u
		base.Path = u.Path[:index]
		base.RawPath = ""
		base.RawQuery = ""
		endpoint, repo, err := NormalizeGitURL(base.String() + ".git")
		if err != nil {
			return Git{}, true, err
		}
		if u.RawQuery != "" || u.ForceQuery {
			return Git{}, true, errors.New("GitLab tree source cannot contain query parameters")
		}
		return Git{URL: endpoint, Repository: repo, Browser: &BrowserTree{Provider: "gitlab", Suffix: u.Path[index+len(marker):]}}, true, nil
	}
	if host == "gitlab.com" {
		if len(segments) < 2 || strings.Contains(u.Path, "/-/") {
			return Git{}, true, errors.New("choose a GitLab repository or tree package root URL")
		}
		copy := *u
		copy.Path = strings.TrimSuffix(copy.Path, "/") + ".git"
		copy.RawPath = ""
		endpoint, repo, err := NormalizeGitURL(copy.String())
		return Git{URL: endpoint, Repository: repo}, true, err
	}
	return Git{}, false, nil
}

func parseAzure(u *url.URL) (Git, error) {
	segments := strings.Split(strings.Trim(u.Path, "/"), "/")
	index := -1
	for i, s := range segments {
		if s == "_git" {
			if index >= 0 {
				return Git{}, errors.New("invalid Azure repository URL")
			}
			index = i
		}
	}
	if index < 1 || index+2 != len(segments) {
		return Git{}, errors.New("Azure source requires an _git repository URL")
	}
	q, err := url.ParseQuery(u.RawQuery)
	if err != nil {
		return Git{}, errors.New("invalid Azure source query")
	}
	for key, values := range q {
		if len(values) != 1 || (key != "path" && key != "version" && key != "_a") {
			return Git{}, errors.New("unsupported Azure source query")
		}
	}
	if q.Get("_a") != "" && q.Get("_a") != "contents" {
		return Git{}, errors.New("choose Azure repository contents")
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
	version := q.Get("version")
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
				return Git{}, errors.New("Azure commit requires a full lowercase SHA")
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
