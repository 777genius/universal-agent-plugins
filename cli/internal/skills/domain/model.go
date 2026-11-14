package domain

type ExecutionMode string

const (
	ExecutionDocsOnly ExecutionMode = "docs_only"
	ExecutionCommand  ExecutionMode = "command"
)

type RuntimeClass string

const (
	RuntimeGo       RuntimeClass = "go"
	RuntimeShell    RuntimeClass = "shell"
	RuntimePython   RuntimeClass = "python"
	RuntimeNode     RuntimeClass = "node"
	RuntimeDeno     RuntimeClass = "deno"
	RuntimeExternal RuntimeClass = "external"
	RuntimeGeneric  RuntimeClass = "generic"
)

type Agent string

const (
	AgentClaude Agent = "claude"
	AgentCodex  Agent = "codex"
)

type CompatibilitySpec struct {
	Requires        []string `yaml:"requires"`
	SupportedOS     []string `yaml:"supported_os"`
	RepoRequired    bool     `yaml:"repo_required"`
	NetworkRequired bool     `yaml:"network_required"`
	Notes           []string `yaml:"notes"`
}

type AgentHint struct {
	Title      string   `yaml:"title"`
	Invocation string   `yaml:"invocation"`
	Notes      []string `yaml:"notes"`
}

type SkillSpec struct {
	Name            string               `yaml:"name"`
	Description     string               `yaml:"description"`
	ExecutionMode   ExecutionMode        `yaml:"execution_mode"`
	SupportedAgents []Agent              `yaml:"supported_agents"`
	AllowedTools    []string             `yaml:"allowed_tools"`
	Command         string               `yaml:"command"`
	Args            []string             `yaml:"args"`
	WorkingDir      string               `yaml:"working_dir"`
	Runtime         RuntimeClass         `yaml:"runtime"`
	Compatibility   CompatibilitySpec    `yaml:"compatibility"`
	Inputs          []string             `yaml:"inputs"`
	Outputs         []string             `yaml:"outputs"`
	SafeToRetry     *bool                `yaml:"safe_to_retry"`
	Timeout         string               `yaml:"timeout"`
	WritesFiles     *bool                `yaml:"writes_files"`
	ProducesJSON    *bool                `yaml:"produces_json"`
	AgentHints      map[string]AgentHint `yaml:"agent_hints"`
}

type SkillDocument struct {
	Spec SkillSpec
	Body string
}

type Artifact struct {
	RelPath string
	Content []byte
}
