package domain

import (
	"fmt"
	"net/url"
	"path"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf16"
	"unicode/utf8"
)

var webSkillSHA256 = regexp.MustCompile(`^[0-9a-f]{64}$`)

// NormalizeWebSkillURL preserves a publisher's path prefix while rejecting
// URL spellings whose decoding could change the discovery directory.
func NormalizeWebSkillURL(value string) (string, error) {
	if !safeWebSkillURLText(value) {
		return "", fmt.Errorf("unsafe HTTPS Skill endpoint")
	}
	u, err := url.Parse(value)
	if err != nil || !webSkillURLCoordinates(u) || strings.Contains(value, "#") {
		return "", fmt.Errorf("Skill endpoint requires credential-free HTTPS without query or fragment")
	}
	if err := validateWebSkillEndpointPath(u); err != nil {
		return "", err
	}
	u.Host = strings.ToLower(u.Host)
	u.Path = strings.TrimSuffix(u.Path, "/")
	u.RawPath = ""
	return u.String(), nil
}

func safeWebSkillURLText(value string) bool {
	return value != "" && len(value) <= 8192 && strings.TrimSpace(value) == value && !strings.ContainsAny(value, `\`) && !strings.ContainsFunc(value, unicode.IsControl)
}

func webSkillURLCoordinates(u *url.URL) bool {
	return u.Scheme == "https" && u.Hostname() != "" && u.User == nil && u.Opaque == "" && u.RawQuery == "" && !u.ForceQuery && u.Fragment == ""
}

func validateWebSkillEndpointPath(u *url.URL) error {
	if strings.ContainsFunc(u.Path, unicode.IsControl) || strings.ContainsAny(u.Path, `\`) {
		return fmt.Errorf("unsafe HTTPS Skill path")
	}
	escaped := strings.ToLower(u.EscapedPath())
	if strings.Contains(escaped, "%2f") || strings.Contains(escaped, "%5c") {
		return fmt.Errorf("encoded Skill path separators are forbidden")
	}
	segments := strings.Split(u.Path, "/")
	for i, segment := range segments {
		if segment == "." || segment == ".." || (segment == "" && i > 0 && i < len(segments)-1) {
			return fmt.Errorf("Skill endpoint path must be normalized")
		}
	}
	return nil
}

// ValidSkillName is the one Skill-name contract shared by every acquisition,
// environment and installer path: 1-64 runes that are Unicode lowercase letters
// or numbers, with single hyphens between them and no leading or trailing hyphen.
func ValidSkillName(name string) bool {
	if size := utf8.RuneCountInString(name); size < 1 || size > 64 || !utf8.ValidString(name) || strings.HasPrefix(name, "-") || strings.HasSuffix(name, "-") || strings.Contains(name, "--") {
		return false
	}
	for _, character := range name {
		if character != '-' && !unicode.IsLower(character) && !unicode.IsNumber(character) {
			return false
		}
	}
	return true
}

// ValidateWebSkillFilePath validates the basic portable path shape. Acquisition
// additionally validates Unicode normalization and complete-tree collisions.
func ValidateWebSkillFilePath(value string) error {
	if value == "" || len(value) > 1024 || strings.Contains(value, "%") || path.IsAbs(value) || path.Clean(value) != value || len(strings.Split(value, "/")) > DefaultMaxDepth {
		return fmt.Errorf("Skill file must have a normalized portable relative path")
	}
	for _, segment := range strings.Split(value, "/") {
		if !validWebSkillPathSegment(segment) {
			return fmt.Errorf("Skill file path is not portable")
		}
	}
	return nil
}

func validWebSkillPathSegment(segment string) bool {
	return segment != "" && segment != "." && segment != ".." && utf8.ValidString(segment) && len(utf16.Encode([]rune(segment))) <= 255 && !strings.ContainsAny(segment, `\/:*?"<>|`) && !strings.HasSuffix(segment, " ") && !strings.HasSuffix(segment, ".") && !strings.ContainsFunc(segment, unicode.IsControl) && !ReservedPhysicalStem(segment)
}

// ValidateWebSkillSourceFacts validates immutable discovery provenance without
// contacting the publisher. Every download coordinate follows the pinned index.
func ValidateWebSkillSourceFacts(facts WebSkillSourceFacts) error {
	base, err := NormalizeWebSkillURL(facts.URL)
	if err != nil || base != facts.URL || !validWebSkillPinIdentity(facts) {
		return fmt.Errorf("invalid HTTPS Skill source facts")
	}
	if facts.IndexURL != base+"/.well-known/agent-skills/index.json" && facts.IndexURL != base+"/.well-known/skills/index.json" {
		return fmt.Errorf("Skill index must belong to its publisher endpoint")
	}
	if len(facts.Files) < 1 || len(facts.Files) > 256 {
		return fmt.Errorf("Skill source must pin 1 to 256 files")
	}
	return validateWebSkillPinnedFiles(facts)
}

func validWebSkillPinIdentity(facts WebSkillSourceFacts) bool {
	return ValidSkillName(facts.Skill) && facts.Normalizer == "agent-skill-package/v1" && webSkillSHA256.MatchString(facts.IndexSHA256) && strings.HasPrefix(facts.RawTreeDigest, "sha256:") && webSkillSHA256.MatchString(strings.TrimPrefix(facts.RawTreeDigest, "sha256:"))
}

func validateWebSkillPinnedFiles(facts WebSkillSourceFacts) error {
	index, _ := url.Parse(facts.IndexURL)
	seen := make(map[string]bool, len(facts.Files))
	hasSkill := false
	for _, file := range facts.Files {
		if err := ValidateWebSkillFilePath(file.Path); err != nil || !webSkillSHA256.MatchString(file.SHA256) {
			return fmt.Errorf("invalid pinned Skill file")
		}
		key := file.Path
		if seen[key] {
			return fmt.Errorf("duplicate pinned Skill file")
		}
		seen[key] = true
		download := *index
		download.Path = path.Dir(index.Path) + "/" + facts.Skill + "/" + file.Path
		download.RawPath = ""
		if file.URL != download.String() {
			return fmt.Errorf("Skill file URL differs from its pinned index directory")
		}
		hasSkill = hasSkill || file.Path == "SKILL.md"
	}
	if !hasSkill {
		return fmt.Errorf("Skill source must pin SKILL.md")
	}
	return validateWebSkillExactParents(seen)
}

func validateWebSkillExactParents(seen map[string]bool) error {
	for key := range seen {
		for parent := path.Dir(key); parent != "."; parent = path.Dir(parent) {
			if seen[parent] {
				return fmt.Errorf("Skill file paths overlap a directory")
			}
		}
	}
	return nil
}

func CloneWebSkillSourceFacts(facts *WebSkillSourceFacts) *WebSkillSourceFacts {
	if facts == nil {
		return nil
	}
	cloned := *facts
	cloned.Files = append([]WebSkillFile(nil), facts.Files...)
	return &cloned
}
