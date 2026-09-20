package app

type PluginExportOptions struct {
	Root     string
	Platform string
	Output   string
}

type PluginExportResult struct {
	Lines []string
}

type exportMetadata struct {
	PluginName         string   `json:"plugin_name"`
	Platform           string   `json:"platform"`
	Runtime            string   `json:"runtime"`
	Manager            string   `json:"manager"`
	BootstrapModel     string   `json:"bootstrap_model"`
	RuntimeRequirement string   `json:"runtime_requirement,omitempty"`
	RuntimeInstallHint string   `json:"runtime_install_hint,omitempty"`
	Next               []string `json:"next"`
	BundleFormat       string   `json:"bundle_format"`
	GeneratedBy        string   `json:"generated_by"`
}
