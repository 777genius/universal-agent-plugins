package publishschema

const (
	APIVersionV1         = "v1"
	CodexMarketplaceRel  = "publish/codex/marketplace.yaml"
	ClaudeMarketplaceRel = "publish/claude/marketplace.yaml"
	GeminiGalleryRel     = "publish/gemini/gallery.yaml"
)

type CodexMarketplace struct {
	APIVersion           string `yaml:"api_version" json:"api_version"`
	MarketplaceName      string `yaml:"marketplace_name" json:"marketplace_name"`
	DisplayName          string `yaml:"display_name,omitempty" json:"display_name,omitempty"`
	SourceRoot           string `yaml:"source_root,omitempty" json:"source_root,omitempty"`
	Category             string `yaml:"category" json:"category"`
	InstallationPolicy   string `yaml:"installation_policy,omitempty" json:"installation_policy,omitempty"`
	AuthenticationPolicy string `yaml:"authentication_policy,omitempty" json:"authentication_policy,omitempty"`
	Path                 string `yaml:"-" json:"path"`
}

type ClaudeMarketplace struct {
	APIVersion      string `yaml:"api_version" json:"api_version"`
	MarketplaceName string `yaml:"marketplace_name" json:"marketplace_name"`
	OwnerName       string `yaml:"owner_name" json:"owner_name"`
	SourceRoot      string `yaml:"source_root,omitempty" json:"source_root,omitempty"`
	Path            string `yaml:"-" json:"path"`
}

type GeminiGallery struct {
	APIVersion           string `yaml:"api_version" json:"api_version"`
	Distribution         string `yaml:"distribution,omitempty" json:"distribution,omitempty"`
	RepositoryVisibility string `yaml:"repository_visibility,omitempty" json:"repository_visibility,omitempty"`
	GitHubTopic          string `yaml:"github_topic,omitempty" json:"github_topic,omitempty"`
	ManifestRoot         string `yaml:"manifest_root,omitempty" json:"manifest_root,omitempty"`
	Path                 string `yaml:"-" json:"path"`
}

type State struct {
	Codex  *CodexMarketplace  `json:"codex,omitempty"`
	Claude *ClaudeMarketplace `json:"claude,omitempty"`
	Gemini *GeminiGallery     `json:"gemini,omitempty"`
}
