package platformmeta

func kimiProfile() PlatformProfile {
	return PlatformProfile{
		ID: "kimi",
		Contract: TargetContractMeta{
			PlatformFamily:         FamilyIDEPlugin,
			TargetClass:            "plugin_package",
			TargetNoun:             "plugin",
			ProductionClass:        "packaging-only target",
			RuntimeContract:        "Kimi Code plugin bundle with portable skills and inline portable MCP",
			InstallModel:           "/plugins install or ~/.kimi-code/plugins/managed copy plus installed.json",
			DevModel:               "package authoring workspace",
			ActivationModel:        "/plugins reload after managed copy",
			NativeRoot:             ".kimi-plugin/plugin.json",
			ImportSupport:          true,
			GenerateSupport:        true,
			ValidateSupport:        true,
			PortableComponentKinds: []string{"skills", "mcp_servers"},
			TargetComponentKinds:   []string{},
			Summary:                "Kimi plugin packages compile portable skills and optional MCP into .kimi-plugin/plugin.json. Kimi requires MCP servers inline in the manifest and does not resolve external MCP file refs. Commands, hooks, agents, and systemPrompt stay unsupported until they have a first-class authoring surface.",
		},
		SDK: SDKMeta{
			PublicPackage:   "kimi",
			InternalPackage: "kimi",
			InternalImport:  "github.com/777genius/plugin-kit-ai/sdk/internal/platforms/kimi",
			Status:          StatusScaffoldOnly,
			TransportModes:  []TransportMode{TransportProcess},
			LiveTestProfile: "kimi_plugin",
		},
		Launcher: LauncherMeta{Requirement: LauncherIgnored},
		SurfaceTiers: []SurfaceSupport{
			{Kind: "commands", Tier: SurfaceTierUnsupported},
			{Kind: "hooks", Tier: SurfaceTierUnsupported},
			{Kind: "agents", Tier: SurfaceTierUnsupported},
			{Kind: "system_prompt", Tier: SurfaceTierUnsupported},
		},
		ManagedArtifacts: []ManagedArtifactSpec{
			{Kind: ManagedArtifactStatic, Path: ".kimi-plugin/plugin.json"},
			{Kind: ManagedArtifactPortableSkills, SourceRoot: authoredPath("skills"), OutputRoot: "skills"},
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
				{Path: authoredPath("README.md"), Template: "kimi.README.md.tmpl"},
				{Path: authoredPath("mcp/servers.yaml"), Template: "mcp.servers.yaml.tmpl", Extra: true},
				{Path: authoredPath("skills/{{.ProjectName}}/SKILL.md"), Template: "SKILL.md.tmpl", Extra: true},
				{Path: "CLAUDE.md", Template: "ROOT.CLAUDE.md.tmpl"},
				{Path: "AGENTS.md", Template: "ROOT.AGENTS.md.tmpl"},
			},
		},
		Validate: ValidateMeta{
			RequiredFiles: []string{
				"README.md",
				".kimi-plugin/plugin.json",
			},
			ForbiddenFiles: []string{
				"launcher.yaml",
			},
		},
	}
}
