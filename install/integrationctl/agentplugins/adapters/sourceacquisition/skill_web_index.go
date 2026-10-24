package sourceacquisition

import (
	"encoding/json"
	"errors"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/conformance"
)

func webSkillIndexFiles(body []byte, exactName string) ([]string, error) {
	if err := conformance.RejectDuplicateJSONKeys(body); err != nil {
		return nil, errors.New("skill index contains duplicate JSON keys")
	}
	var index struct {
		Schema  string `json:"$schema"`
		Version string `json:"version"`
		Skills  []struct {
			Name  string   `json:"name"`
			Files []string `json:"files"`
		} `json:"skills"`
	}
	if err := json.Unmarshal(body, &index); err != nil || index.Skills == nil {
		return nil, errors.New("malformed Skill index: expected a skills array")
	}
	if index.Schema != "" || index.Version != "" {
		return nil, errors.New("unsupported versioned Skill index dialect: only the unversioned complete files list is supported")
	}
	var selected []string
	seenNames := map[string]bool{}
	for _, entry := range index.Skills {
		if entry.Name == "" || seenNames[entry.Name] {
			return nil, errors.New("skill index has empty or duplicate Skill names")
		}
		seenNames[entry.Name] = true
		if entry.Name == exactName {
			selected = entry.Files
		}
	}
	if !seenNames[exactName] {
		return nil, errors.New("named Skill is absent from publisher index")
	}
	if len(selected) == 0 {
		return nil, errors.New("unsupported Skill index dialect: a complete files list is required; archive and skill-md entries are unsupported")
	}
	return selected, nil
}
