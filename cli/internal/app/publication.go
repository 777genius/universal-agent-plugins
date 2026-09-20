package app

type PluginPublicationMaterializeOptions struct {
	Root        string
	Target      string
	Dest        string
	PackageRoot string
	DryRun      bool
}

type PluginPublicationMaterializeResult struct {
	Target            string            `json:"target"`
	Mode              string            `json:"mode"`
	MarketplaceFamily string            `json:"marketplace_family"`
	Dest              string            `json:"dest"`
	PackageRoot       string            `json:"package_root"`
	Details           map[string]string `json:"details"`
	NextSteps         []string          `json:"next_steps"`
	Lines             []string          `json:"-"`
}

type PluginPublicationRemoveOptions struct {
	Root        string
	Target      string
	Dest        string
	PackageRoot string
	DryRun      bool
}

type PluginPublicationRemoveResult struct {
	Lines []string
}

type PluginPublicationVerifyRootOptions struct {
	Root        string
	Target      string
	Dest        string
	PackageRoot string
}

type PluginPublicationRootIssue struct {
	Code    string `json:"code"`
	Path    string `json:"path,omitempty"`
	Message string `json:"message"`
}

type PluginPublicationVerifyRootResult struct {
	Ready       bool                         `json:"ready"`
	Status      string                       `json:"status"`
	Dest        string                       `json:"dest"`
	PackageRoot string                       `json:"package_root"`
	CatalogPath string                       `json:"catalog_path"`
	IssueCount  int                          `json:"issue_count"`
	Issues      []PluginPublicationRootIssue `json:"issues"`
	NextSteps   []string                     `json:"next_steps"`
	Lines       []string                     `json:"-"`
}

type PluginPublishOptions struct {
	Root        string
	Channel     string
	Dest        string
	PackageRoot string
	DryRun      bool
	All         bool
}

type PluginPublishIssue struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type PluginPublishResult struct {
	Channel       string                `json:"channel,omitempty"`
	Target        string                `json:"target,omitempty"`
	Ready         bool                  `json:"ready"`
	Status        string                `json:"status"`
	Mode          string                `json:"mode"`
	WorkflowClass string                `json:"workflow_class"`
	Dest          string                `json:"dest,omitempty"`
	PackageRoot   string                `json:"package_root,omitempty"`
	Details       map[string]string     `json:"details"`
	IssueCount    int                   `json:"issue_count"`
	Issues        []PluginPublishIssue  `json:"issues"`
	WarningCount  int                   `json:"warning_count,omitempty"`
	Warnings      []string              `json:"warnings,omitempty"`
	NextSteps     []string              `json:"next_steps"`
	ChannelCount  int                   `json:"channel_count,omitempty"`
	Channels      []PluginPublishResult `json:"channels,omitempty"`
	Lines         []string              `json:"-"`
}

func (service PluginService) Publish(opts PluginPublishOptions) (PluginPublishResult, error) {
	return service.publish(opts)
}

func (service PluginService) PublicationMaterialize(opts PluginPublicationMaterializeOptions) (PluginPublicationMaterializeResult, error) {
	return service.publicationMaterialize(opts)
}

func (service PluginService) PublicationRemove(opts PluginPublicationRemoveOptions) (PluginPublicationRemoveResult, error) {
	return service.publicationRemove(opts)
}

func (service PluginService) PublicationVerifyRoot(opts PluginPublicationVerifyRootOptions) (PluginPublicationVerifyRootResult, error) {
	return service.publicationVerifyRoot(opts)
}
