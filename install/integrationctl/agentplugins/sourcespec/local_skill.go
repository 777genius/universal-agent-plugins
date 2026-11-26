package sourcespec

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"path"
	"strings"
	"unicode"
	"unicode/utf8"
)

const LocalSkillKind = "local-skill"

// ValidateLocalSkillPath checks serialized absolute coordinates independently
// of the reading OS. Reopening a source also requires a native absolute path.
func ValidateLocalSkillPath(value string) error {
	if value == "" || len(value) > 8192 || !utf8.ValidString(value) || strings.TrimSpace(value) != value || strings.Contains(value, `\`) || strings.ContainsFunc(value, unicode.IsControl) {
		return errors.New("local Skill source requires a canonical absolute path using forward slashes")
	}
	clean := value
	switch {
	case strings.HasPrefix(value, "//"):
		parts := strings.Split(strings.TrimPrefix(value, "//"), "/")
		if len(parts) < 2 || parts[0] == "" || parts[1] == "" {
			return errors.New("local Skill UNC source requires a server and share")
		}
		clean = "/" + strings.TrimPrefix(value, "//")
	case len(value) >= 3 && asciiLetter(value[0]) && value[1:3] == ":/":
		clean = value[2:]
	case !strings.HasPrefix(value, "/"):
		return errors.New("local Skill source requires an absolute path")
	}
	if path.Clean(clean) != clean {
		return errors.New("local Skill source path must be canonical")
	}
	return nil
}

func asciiLetter(value byte) bool {
	return value >= 'A' && value <= 'Z' || value >= 'a' && value <= 'z'
}

// LocalSkillKey keeps ownership independent of mutable source content.
func LocalSkillKey(absolutePath, skill string) string {
	body, _ := json.Marshal([3]string{"local-skill-v1", absolutePath, skill})
	sum := sha256.Sum256(body)
	return "local-skill-v1:" + hex.EncodeToString(sum[:])
}
