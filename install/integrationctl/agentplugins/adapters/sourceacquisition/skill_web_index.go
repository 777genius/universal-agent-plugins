package sourceacquisition

import (
	"encoding/json"
	"errors"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/conformance"
)

type webSkillIndexEntry struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Files       []string `json:"files"`
}

type webSkillIndex struct {
	Schema  string               `json:"$schema"`
	Version string               `json:"version"`
	Skills  []webSkillIndexEntry `json:"skills"`
}

func decodeWebSkillIndex(body []byte) (webSkillIndex, error) {
	var index webSkillIndex
	if err := conformance.RejectDuplicateJSONKeys(body); err != nil {
		return index, errors.New("skill index contains duplicate JSON keys")
	}
	if err := json.Unmarshal(body, &index); err != nil || index.Skills == nil {
		return index, errors.New("malformed Skill index: expected a skills array")
	}
	if index.Schema != "" || index.Version != "" {
		return index, errors.New("unsupported versioned Skill index dialect: only the unversioned complete files list is supported")
	}
	seen := map[string]bool{}
	for _, entry := range index.Skills {
		if entry.Name == "" || seen[entry.Name] {
			return index, errors.New("skill index has empty or duplicate Skill names")
		}
		seen[entry.Name] = true
	}
	return index, nil
}
func webSkillIndexFiles(body []byte, exactName string) ([]string, error) {
	index, err := decodeWebSkillIndex(body)
	if err != nil {
		return nil, err
	}
	for _, entry := range index.Skills {
		if entry.Name != exactName {
			continue
		}
		if len(entry.Files) == 0 {
			return nil, errors.New("unsupported Skill index dialect: a complete files list is required; archive and skill-md entries are unsupported")
		}
		return entry.Files, nil
	}
	return nil, errors.New("named Skill is absent from publisher index")
}
