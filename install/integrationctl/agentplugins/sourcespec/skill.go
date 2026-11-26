package sourcespec

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
)

// GitSkillKey identifies a normalized raw Skill independently of its revision
// and resolved location. The tuple keeps endpoint and path boundaries exact.
func GitSkillKey(endpoint, name, requestedPath string) string {
	body, _ := json.Marshal([4]string{"git-skill-v1", endpoint, name, requestedPath})
	sum := sha256.Sum256(body)
	return "git-skill-url-v1:" + hex.EncodeToString(sum[:])
}
