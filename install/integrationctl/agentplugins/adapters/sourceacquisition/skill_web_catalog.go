package sourceacquisition

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

type HTTPSkillCandidate struct {
	Name, Description, Reason string
	Available                 bool
}
type HTTPSkillCatalog struct {
	URL, IndexSHA256 string
	Candidates       []HTTPSkillCandidate
}

// ListHTTPSkills reads the same bounded, validated publisher index as exact
// acquisition. No Skill files or persistent installer state are written.
func (acquirer Acquirer) ListHTTPSkills(ctx context.Context, endpoint string) (HTTPSkillCatalog, error) {
	base, err := domain.NormalizeWebSkillURL(endpoint)
	if err != nil {
		return HTTPSkillCatalog{}, err
	}
	ctx, cancel := context.WithTimeout(ctx, webSkillTimeout)
	defer cancel()
	client := acquirer.webSkillClient()
	defer client.CloseIdleConnections()
	indexURL, body, err := loadWebSkillIndex(ctx, client, base)
	if err != nil {
		return HTTPSkillCatalog{}, err
	}
	index, err := decodeWebSkillIndex(body)
	if err != nil {
		return HTTPSkillCatalog{}, err
	}
	if len(index.Skills) > 2000 {
		return HTTPSkillCatalog{}, errors.New("publisher Skill catalog exceeds candidate limit")
	}
	catalog := HTTPSkillCatalog{URL: base, IndexSHA256: webSkillSHA256(body), Candidates: []HTTPSkillCandidate{}}
	for _, entry := range index.Skills {
		candidate := HTTPSkillCandidate{Name: entry.Name, Description: entry.Description, Available: true}
		if _, err := webSkillFacts(base, indexURL, body, entry.Name, entry.Files); err != nil {
			candidate.Available = false
			candidate.Reason = "Publisher entry has an incomplete or unsafe file list"
		}
		catalog.Candidates = append(catalog.Candidates, candidate)
	}
	return catalog, nil
}
func loadWebSkillIndex(ctx context.Context, client *http.Client, base string) (string, []byte, error) {
	indexURL := base + "/.well-known/agent-skills/index.json"
	body, status, contentType, err := webSkillGET(ctx, client, indexURL, webSkillIndexBytes)
	if err == nil && (status == http.StatusNotFound || status == http.StatusOK && webSkillHTML(body, contentType)) {
		indexURL = base + "/.well-known/skills/index.json"
		body, status, _, err = webSkillGET(ctx, client, indexURL, webSkillIndexBytes)
	}
	if err != nil {
		return "", nil, err
	}
	if status != http.StatusOK {
		return "", nil, fmt.Errorf("skill index returned HTTP %d", status)
	}
	return indexURL, body, nil
}
