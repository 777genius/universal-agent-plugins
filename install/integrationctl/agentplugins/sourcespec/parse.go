package sourcespec

import (
	"errors"
	"net/url"
	"strings"
	"unicode"
)

// Parse recognizes direct inputs only. Directory names and local directories
// remain the caller's grammar; no filesystem existence probe is performed.
func Parse(raw string) (Spec, error) {
	if spec, recognized, err := parseLocalArchiveSelector(raw); recognized {
		return spec, err
	}
	if explicitArchivePath(raw) {
		if len(raw) > 8192 || strings.TrimSpace(raw) != raw || strings.ContainsFunc(raw, unicode.IsControl) {
			return Spec{}, errors.New("invalid local archive path")
		}
		return Spec{Requested: raw, Kind: KindLocalArchive, Archive: &Archive{URL: raw}}, nil
	}
	if !safeText(raw) {
		return Spec{}, errors.New("invalid source input")
	}
	out := Spec{Requested: raw}
	if !strings.ContainsAny(raw, "/\\:") && HasArchiveSuffix(raw) {
		out.Kind, out.Archive = KindLocalArchive, &Archive{URL: raw}
		return out, nil
	}
	if strings.HasPrefix(raw, "github:") || (!strings.Contains(raw, ":") && strings.Contains(raw, "@")) {
		g, err := parseShorthand(raw)
		out.Kind, out.Git = KindGit, &g
		return out, err
	}
	if strings.HasPrefix(raw, "git+https://") || strings.HasPrefix(raw, "ssh://") || looksLikeSCP(raw) {
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
	return parseHTTPS(raw, out)
}

func explicitArchivePath(raw string) bool {
	if strings.Contains(raw, "://") || !HasArchiveSuffix(raw) {
		return false
	}
	return strings.HasPrefix(raw, "./") || strings.HasPrefix(raw, "../") || strings.HasPrefix(raw, "/") || strings.ContainsRune(raw, '\\') || (len(raw) >= 3 && raw[1] == ':' && raw[2] == '/')
}

func looksLikeSCP(raw string) bool {
	if strings.Contains(raw, "://") {
		return false
	}
	host, _, ok := strings.Cut(raw, ":")
	return ok && !strings.ContainsAny(host, "/\\") && (strings.Contains(host, "@") || strings.Contains(host, "."))
}

func parseHTTPS(raw string, out Spec) (Spec, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return Spec{}, errors.New("invalid HTTPS source")
	}
	if strings.HasPrefix(u.Fragment, "sha256=") || HasArchiveSuffix(u.Path) || IsProviderArchiveURL(u) {
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
	base := *u
	base.Fragment = ""
	endpoint, err := NormalizeArchiveURL(base.String())
	if err != nil {
		return Archive{}, err
	}
	return parseArchiveFragment(Archive{URL: endpoint}, fragment)
}

func parseArchiveFragment(a Archive, fragment string) (Archive, error) {
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

func parseLocalArchiveSelector(raw string) (Spec, bool, error) {
	base, fragment, selected := strings.Cut(raw, "#")
	if !selected || !strings.HasPrefix(fragment, "sha256=") || !HasArchiveSuffix(base) || strings.Contains(base, "://") || looksLikeSCP(base) || strings.HasPrefix(base, "github:") {
		return Spec{}, false, nil
	}
	if len(raw) > 8192 || strings.TrimSpace(raw) != raw || strings.ContainsFunc(raw, unicode.IsControl) {
		return Spec{}, true, errors.New("invalid local archive selector")
	}
	archive, err := parseArchiveFragment(Archive{URL: base}, fragment)
	return Spec{Requested: raw, Kind: KindLocalArchive, Archive: &archive}, true, err
}
