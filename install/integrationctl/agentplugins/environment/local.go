package environment

import (
	"errors"
	"path"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/sourcespec"
)

// LocalSource is explicit, root-only snapshot intent. Path is serialized in
// canonical absolute forward-slash form; it never names a live installation.
type LocalSource struct {
	Path  string `json:"path"`
	Skill string `json:"skill"`
}

type LockedLocalSource struct {
	Path          string `json:"path"`
	Skill         string `json:"skill"`
	RawTreeDigest string `json:"raw_tree_digest"`
	Normalizer    string `json:"normalizer"`
}

func (source LocalSource) Validate() error {
	if err := sourcespec.ValidateLocalSkillPath(source.Path); err != nil {
		return err
	}
	if path.Base(source.Path) != source.Skill {
		return errors.New("local Skill name must match its source directory")
	}
	return validateSkillName(source.Skill)
}

func validateLockedLocal(requirement Requirement, pkg LockedPackage) error {
	source := pkg.Source.Local
	if source == nil || source.Path != requirement.Local.Path || source.Skill != requirement.Local.Skill || source.Normalizer != SkillNormalizerV1 || !validGraphDigest(source.RawTreeDigest) {
		return errors.New("locked local Skill identity does not match manifest")
	}
	if len(pkg.RequiredBy) != 0 {
		return errors.New("local Skills are root-only dependencies")
	}
	return nil
}
