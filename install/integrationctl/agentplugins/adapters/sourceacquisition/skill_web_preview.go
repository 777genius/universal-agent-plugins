package sourceacquisition

import (
	"context"
	"errors"
	"unicode/utf8"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// PreviewHTTPSkill reads the publisher-declared SKILL.md without installing,
// executing, normalizing, or downloading companion files. Its discovery and
// path validation are shared with the real HTTPS Skill installer.
func (acquirer Acquirer) PreviewHTTPSkill(ctx context.Context, endpoint, name string) (string, error) {
	base, err := domain.NormalizeWebSkillURL(endpoint)
	if err != nil || !domain.ValidSkillName(name) {
		return "", errors.New("invalid website Skill preview")
	}
	ctx, cancel := context.WithTimeout(ctx, webSkillTimeout)
	defer cancel()
	client := acquirer.webSkillClient()
	defer client.CloseIdleConnections()
	indexURL, index, err := loadWebSkillIndex(ctx, client, base)
	if err != nil {
		return "", err
	}
	files, err := webSkillIndexFiles(index, name)
	if err != nil {
		return "", err
	}
	facts, err := webSkillFacts(base, indexURL, index, name, files)
	if err != nil {
		return "", err
	}
	for _, file := range facts.Files {
		if file.Path != "SKILL.md" {
			continue
		}
		body, status, _, fetchErr := webSkillGET(ctx, client, file.URL, 64<<10)
		if fetchErr != nil {
			return "", fetchErr
		}
		if status != 200 || !utf8.Valid(body) || len(body) == 0 {
			return "", errors.New("publisher Skill document is unavailable or invalid")
		}
		return string(body), nil
	}
	return "", errors.New("publisher has no SKILL.md")
}
