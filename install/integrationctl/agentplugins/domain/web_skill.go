package domain

// WebSkillFile pins one complete publisher-provided file. URL is the exact
// download coordinate; Path is its portable path in the raw Skill tree.
type WebSkillFile struct {
	Path   string `json:"path"`
	URL    string `json:"url"`
	SHA256 string `json:"sha256"`
}

// WebSkillSourceFacts preserves discovery provenance and the immutable files
// needed to restore a Skill without consulting a mutable publisher index.
type WebSkillSourceFacts struct {
	URL           string         `json:"url"`
	Skill         string         `json:"skill"`
	IndexURL      string         `json:"index_url"`
	IndexSHA256   string         `json:"index_sha256"`
	Files         []WebSkillFile `json:"files"`
	RawTreeDigest string         `json:"raw_tree_digest"`
	Normalizer    string         `json:"normalizer"`
}
