package sourceacquisition

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/packagedigest"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

const (
	webSkillIndexBytes = int64(1 << 20)
	webSkillFileBytes  = int64(8 << 20)
	webSkillTreeBytes  = int64(32 << 20)
	webSkillTimeout    = 30 * time.Second
)

// ResolveHTTPSkill discovers exactly one named Skill and retains every listed
// file. Its sealed snapshot contains the raw Skill, without generated metadata.
// The caller owns the snapshot through packagedigest.Remove.
func (acquirer Acquirer) ResolveHTTPSkill(ctx context.Context, endpoint, exactName string) (domain.PackageSnapshot, domain.WebSkillSourceFacts, error) {
	var facts domain.WebSkillSourceFacts
	base, err := domain.NormalizeWebSkillURL(endpoint)
	if err != nil {
		return domain.PackageSnapshot{}, facts, err
	}
	ctx, cancel := context.WithTimeout(ctx, webSkillTimeout)
	defer cancel()
	indexURL := base + "/.well-known/agent-skills/index.json"
	body, status, contentType, err := acquirer.webSkillGET(ctx, indexURL, webSkillIndexBytes)
	if err == nil && (status == http.StatusNotFound || status == http.StatusOK && webSkillHTML(body, contentType)) {
		indexURL = base + "/.well-known/skills/index.json"
		body, status, _, err = acquirer.webSkillGET(ctx, indexURL, webSkillIndexBytes)
	}
	if err != nil {
		return domain.PackageSnapshot{}, facts, err
	}
	if status != http.StatusOK {
		return domain.PackageSnapshot{}, facts, fmt.Errorf("skill index returned HTTP %d", status)
	}
	selected, err := webSkillIndexFiles(body, exactName)
	if err != nil {
		return domain.PackageSnapshot{}, facts, err
	}
	facts = domain.WebSkillSourceFacts{URL: base, Skill: exactName, IndexURL: indexURL, IndexSHA256: webSkillSHA256(body), Normalizer: "agent-skill-package/v1", RawTreeDigest: "sha256:" + strings.Repeat("0", 64)}
	indexCoordinate, _ := url.Parse(indexURL)
	for _, relative := range selected {
		fileURL := *indexCoordinate
		fileURL.Path = path.Dir(indexCoordinate.Path) + "/" + exactName + "/" + relative
		fileURL.RawPath = ""
		facts.Files = append(facts.Files, domain.WebSkillFile{Path: relative, URL: fileURL.String(), SHA256: strings.Repeat("0", 64)})
	}
	sort.Slice(facts.Files, func(i, j int) bool { return facts.Files[i].Path < facts.Files[j].Path })
	if err := domain.ValidateWebSkillSourceFacts(facts); err != nil {
		return domain.PackageSnapshot{}, domain.WebSkillSourceFacts{}, err
	}
	if err := validateWebSkillTreePaths(facts.Files); err != nil {
		return domain.PackageSnapshot{}, domain.WebSkillSourceFacts{}, err
	}
	snapshot, err := acquirer.webSkillRetain(ctx, &facts, false)
	if err != nil {
		return domain.PackageSnapshot{}, domain.WebSkillSourceFacts{}, err
	}
	facts.RawTreeDigest = snapshot.TreeDigest
	return snapshot, facts, nil
}

// AcquireHTTPSkillVerified restores exact pinned file coordinates. It never
// re-fetches or resolves the mutable discovery index.
func (acquirer Acquirer) AcquireHTTPSkillVerified(ctx context.Context, facts domain.WebSkillSourceFacts) (domain.PackageSnapshot, error) {
	if err := domain.ValidateWebSkillSourceFacts(facts); err != nil {
		return domain.PackageSnapshot{}, err
	}
	if err := validateWebSkillTreePaths(facts.Files); err != nil {
		return domain.PackageSnapshot{}, err
	}
	// Avoid changing a caller-owned files slice while checking retained pins.
	facts.Files = append([]domain.WebSkillFile(nil), facts.Files...)
	ctx, cancel := context.WithTimeout(ctx, webSkillTimeout)
	defer cancel()
	return acquirer.webSkillRetain(ctx, &facts, true)
}

func (acquirer Acquirer) webSkillRetain(ctx context.Context, facts *domain.WebSkillSourceFacts, verify bool) (domain.PackageSnapshot, error) {
	workspace, err := os.MkdirTemp(acquirer.TempRoot, "agentplugins-web-skill-*")
	if err != nil {
		return domain.PackageSnapshot{}, errors.New("create private Skill workspace")
	}
	defer func() { _ = os.RemoveAll(workspace) }()
	var total int64
	for i, file := range facts.Files {
		body, status, _, err := acquirer.webSkillGET(ctx, file.URL, min(webSkillFileBytes, webSkillTreeBytes-total))
		if err != nil {
			return domain.PackageSnapshot{}, err
		}
		if status != http.StatusOK {
			return domain.PackageSnapshot{}, fmt.Errorf("skill file returned HTTP %d", status)
		}
		total += int64(len(body))
		checksum := webSkillSHA256(body)
		if verify && checksum != file.SHA256 {
			return domain.PackageSnapshot{}, errors.New("skill file original-byte SHA256 does not match its locked checksum")
		}
		facts.Files[i].SHA256 = checksum
		filename := filepath.Join(workspace, filepath.FromSlash(file.Path))
		if err := os.MkdirAll(filepath.Dir(filename), 0o700); err != nil {
			return domain.PackageSnapshot{}, errors.New("create private Skill file parent")
		}
		if err := os.WriteFile(filename, body, 0o600); err != nil {
			return domain.PackageSnapshot{}, errors.New("write private Skill file")
		}
	}
	snapshot, err := acquirer.digester().SnapshotWithExecutables(ctx, workspace, domain.SourceIdentity{}, nil)
	if err != nil {
		return domain.PackageSnapshot{}, fmt.Errorf("seal acquired Skill tree: %w", err)
	}
	if verify && snapshot.TreeDigest != facts.RawTreeDigest {
		_ = packagedigest.Remove(snapshot)
		return domain.PackageSnapshot{}, errors.New("skill raw tree digest does not match its locked digest")
	}
	facts.RawTreeDigest = snapshot.TreeDigest
	key := "https-skill:" + facts.URL + ":" + facts.Skill
	snapshot.Source = domain.SourceIdentity{
		SourceKind: "https-skill", SourceURL: facts.URL, WebSkill: domain.CloneWebSkillSourceFacts(facts),
		StableSourceKey: key, RequestedSource: key, CanonicalSource: key,
		ResolvedRevision: snapshot.TreeDigest, SourceBindingHint: "uap-dependency-web-skill",
	}
	snapshot.AcquiredAt = acquirer.now()
	return snapshot, nil
}

func webSkillSHA256(body []byte) string {
	digest := sha256.Sum256(body)
	return hex.EncodeToString(digest[:])
}

// webSkillGET inherits only the host's TLS/proxy transport. Cookies, automatic
// authentication, content encodings and all redirects are deliberately refused.
func (acquirer Acquirer) webSkillGET(ctx context.Context, endpoint string, limit int64) ([]byte, int, string, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, http.NoBody)
	if err != nil {
		return nil, 0, "", errors.New("create Skill HTTPS request")
	}
	request.Header.Set("Accept-Encoding", "identity")
	client := acquirer.archiveHTTPClient()
	defer client.CloseIdleConnections()
	client.Timeout = min(client.Timeout, webSkillTimeout)
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	response, err := client.Do(request)
	if err != nil {
		if ctx.Err() != nil {
			return nil, 0, "", ctx.Err()
		}
		return nil, 0, "", errors.New("skill HTTPS request failed (check TLS and connectivity)")
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode != http.StatusOK {
		return nil, response.StatusCode, response.Header.Get("Content-Type"), nil
	}
	if encoding := response.Header.Get("Content-Encoding"); encoding != "" && encoding != "identity" {
		return nil, 0, "", errors.New("skill download refuses HTTP content encoding")
	}
	if response.ContentLength > limit {
		return nil, 0, "", errors.New("skill download exceeds its byte limit")
	}
	body, err := io.ReadAll(io.LimitReader(&archiveContextReader{ctx: ctx, reader: response.Body}, limit+1))
	if err != nil {
		if ctx.Err() != nil {
			return nil, 0, "", ctx.Err()
		}
		return nil, 0, "", errors.New("skill download was incomplete")
	}
	if int64(len(body)) > limit {
		return nil, 0, "", errors.New("skill download exceeds its byte limit")
	}
	return body, response.StatusCode, response.Header.Get("Content-Type"), nil
}

// Publishers sometimes serve their ordinary HTML app at unknown paths with
// HTTP 200. This positive HTML signal permits the legacy index probe; malformed
// JSON, permission failures and all other responses remain terminal.
func webSkillHTML(body []byte, contentType string) bool {
	mediaType := strings.TrimSpace(strings.SplitN(contentType, ";", 2)[0])
	prefix := strings.ToLower(strings.TrimSpace(string(body)))
	return strings.EqualFold(mediaType, "text/html") || strings.HasPrefix(prefix, "<!doctype html") || strings.HasPrefix(prefix, "<html")
}
