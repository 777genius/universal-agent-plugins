package environment

import (
	"errors"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// NormalizeWebURL canonicalizes the publisher endpoint used by portable intent.
func NormalizeWebURL(endpoint string) (string, error) { return domain.NormalizeWebSkillURL(endpoint) }

func (source WebSource) Validate() error {
	endpoint, err := NormalizeWebURL(source.URL)
	if err != nil || endpoint != source.URL {
		return errors.New("website Skill source requires a normalized credential-free HTTPS URL")
	}
	return validateSkillName(source.Skill)
}

func validateLockedWeb(requirement Requirement, pkg LockedPackage) error {
	source := pkg.Source.Web
	if source == nil || pkg.Source.Git != nil || pkg.Source.Registry != nil || pkg.Source.Archive != nil || source.URL != requirement.Web.URL || source.Skill != requirement.Web.Skill {
		return errors.New("locked website Skill identity does not match manifest")
	}
	return domain.ValidateWebSkillSourceFacts(*source)
}
