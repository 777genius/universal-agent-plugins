package platformmeta

func grokProfile() PlatformProfile {
	return PlatformProfile{
		ID: "grok",
		Contract: TargetContractMeta{
			PlatformFamily:         FamilyIDEPlugin,
			TargetClass:            "plugin_package",
			TargetNoun:             "plugin",
			ProductionClass:        "packaging-only target",
			RuntimeContract:        "Grok Build plugin bundle with portable skills and optional portable MCP",
			InstallModel:           "grok plugin install --trust or ~/.grok/plugins copy",
			DevModel:               "package authoring workspace",
			ActivationModel:        "grok plugin list --json or plugin reload",
			NativeRoot:             ".grok-plugin/plugin.json",
			ImportSupport:          true,
			GenerateSupport:        true,
			ValidateSupport:        true,
			PortableComponentKinds: []string{"skills", "mcp_servers"},
			TargetComponentKinds:   []string{},
			Summary:                "Grok plugin packages compile portable skills and optional MCP into the observed .grok-plugin bundle shape. MCP is projected as a wrapped .mcp.json sidecar; hooks, agents, commands, and LSP stay unsupported until they have a first-class authoring surface.",
		},
		SDK: SDKMeta{
			PublicPackage:   "grok",
			InternalPackage: "grok",
			InternalImport:  "github.com/777genius/plugin-kit-ai/sdk/internal/platforms/grok",
			Status:          StatusScaffoldOnly,
			TransportModes:  []TransportMode{TransportProcess},
			LiveTestProfile: "grok_plugin",
		},
		Launcher: LauncherMeta{Requirement: LauncherIgnored},
		SurfaceTiers: []SurfaceSupport{
			{Kind: "hooks", Tier: SurfaceTierUnsupported},
			{Kind: "agents", Tier: SurfaceTierUnsupported},
			{Kind: "commands", Tier: SurfaceTierUnsupported},
			{Kind: "lsp", Tier: SurfaceTierUnsupported},
		},
		ManagedArtifacts: []ManagedArtifactSpec{
			{Kind: ManagedArtifactStatic, Path: ".grok-plugin/plugin.json"},
			{Kind: ManagedArtifactPortableSkills, SourceRoot: authoredPath("skills"), OutputRoot: "skills"},
			{Kind: ManagedArtifactPortableMCP, Path: ".mcp.json"},
		},
		Scaffold: ScaffoldMeta{
			RequiredFiles: []string{
				authoredPath("plugin.yaml"),
				authoredPath("README.md"),
				"CLAUDE.md",
				"AGENTS.md",
			},
			OptionalFiles: []string{
				authoredPath("mcp/servers.yaml"),
				authoredPath("skills/{{.ProjectName}}/SKILL.md"),
			},
			ForbiddenFiles: []string{
				"launcher.yaml",
			},
			TemplateFiles: []TemplateFile{
				{Path: authoredPath("plugin.yaml"), Template: "plugin.yaml.tmpl"},
				{Path: authoredPath("README.md"), Template: "grok.README.md.tmpl"},
				{Path: authoredPath("mcp/servers.yaml"), Template: "mcp.servers.yaml.tmpl", Extra: true},
				{Path: authoredPath("skills/{{.ProjectName}}/SKILL.md"), Template: "SKILL.md.tmpl", Extra: true},
				{Path: "CLAUDE.md", Template: "ROOT.CLAUDE.md.tmpl"},
				{Path: "AGENTS.md", Template: "ROOT.AGENTS.md.tmpl"},
			},
		},
		Validate: ValidateMeta{
			RequiredFiles: []string{
				"README.md",
				".grok-plugin/plugin.json",
			},
			ForbiddenFiles: []string{
				"launcher.yaml",
			},
		},
	}
}
