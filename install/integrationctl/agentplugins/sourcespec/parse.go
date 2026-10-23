package sourcespec

import (
	"errors"
	"net/url"
	"strings"
)

// Parse recognizes direct inputs only. Directory names and local directories
// remain the caller's grammar; no filesystem existence probe is performed.
func Parse(raw string) (Spec, error) {
	if !safeText(raw) {
		return Spec{}, errors.New("invalid source input")
	}
	out := Spec{Requested: raw}
	if strings.HasPrefix(raw, "github:") || (!strings.Contains(raw, ":") && strings.Contains(raw, "@")) {
		g, err := parseShorthand(raw)
		out.Kind, out.Git = KindGit, &g
		return out, err
	}
	if strings.HasPrefix(raw, "git+https://") || strings.HasPrefix(raw, "ssh://") || (!strings.Contains(raw, "://") && strings.Contains(raw, "@") && strings.Contains(raw, ":")) {
		g, err := parseGit(raw)
		out.Kind, out.Git = KindGit, &g
		return out, err
	}
	if !strings.HasPrefix(raw, "https://") {
		if HasArchiveSuffix(raw) {
			out.Kind = KindLocalArchive
			out.Archive = &Archive{URL: raw}
			return out, nil
		}
		return Spec{}, errors.New("not a direct remote or archive source")
	}
	u, err := url.Parse(raw)
	if err != nil {
		return Spec{}, errors.New("invalid HTTPS source")
	}
	if strings.HasPrefix(u.Fragment, "sha256=") || HasArchiveSuffix(u.Path) {
		a, err := parseArchive(u)
		out.Kind, out.Archive = KindArchive, &a
		return out, err
	}
	if isAzureHost(strings.ToLower(u.Hostname())) && (u.RawQuery != "" || u.ForceQuery) {
		g, _, err := parseProvider(u)
		out.Kind, out.Git = KindGit, &g
		return out, err
	}
	if u.Fragment != "" || strings.HasSuffix(u.Path, ".git") || strings.Contains(u.Path, ".git//") {
		g, err := parseGit(raw)
		out.Kind, out.Git = KindGit, &g
		return out, err
	}
	if g, recognized, err := parseProvider(u); recognized {
		out.Kind, out.Git = KindGit, &g
		return out, err
	}
	endpoint, err := NormalizeArchiveURL(raw)
	out.Kind = KindHTTPSAuto
	out.Archive = &Archive{URL: endpoint}
	return out, err
}

func parseArchive(u *url.URL) (Archive, error) {
	fragment := u.Fragment
	copy := *u
	copy.Fragment = ""
	endpoint, err := NormalizeArchiveURL(copy.String())
	if err != nil {
		return Archive{}, err
	}
	a := Archive{URL: endpoint}
	if fragment != "" {
		values, err := url.ParseQuery(fragment)
		if err != nil || len(values["sha256"]) != 1 || !ValidSHA256(values.Get("sha256")) {
			return Archive{}, errors.New("archive fragment requires sha256=FULL_SHA256 and optional path")
		}
		for key, value := range values {
			if (key != "sha256" && key != "path") || len(value) != 1 {
				return Archive{}, errors.New("invalid archive selector")
			}
		}
		a.SHA256 = values.Get("sha256")
		a.Path, err = NormalizeSubpath(values.Get("path"))
		if err != nil {
			return Archive{}, err
		}
	}
	return a, nil
}
