package platformmeta

func qoderProfile() PlatformProfile {
	return PlatformProfile{
		ID: "qoder",
		Contract: TargetContractMeta{
			PlatformFamily:         FamilyIDEPlugin,
			TargetClass:            "plugin_package",
			TargetNoun:             "plugin",
			ProductionClass:        "production-ready package+runtime lane",
			RuntimeContract:        "Qoder plugin bundle with portable skills and optional portable MCP plus target-native hooks and commands, shared by Qoder Desktop and Qoder CLI",
			InstallModel:           "qoder plugins install or Desktop marketplace install",
			DevModel:               "package authoring workspace",
			ActivationModel:        "qoder plugins reload or restart",
			NativeRoot:             ".qoder-plugin/plugin.json",
			ImportSupport:          true,
			GenerateSupport:        true,
			ValidateSupport:        true,
			PortableComponentKinds: []string{"skills", "mcp_servers"},
			TargetComponentKinds:   []string{"hooks", "commands"},
			Summary:                "Qoder plugin packages compile portable skills and optional portable MCP plus target-native hooks and commands into the observed .qoder-plugin bundle shape consumed by both Qoder Desktop and Qoder CLI.",
		},
		SDK: SDKMeta{
			PublicPackage:   "qoder",
			InternalPackage: "qoder",
			InternalImport:  "github.com/777genius/plugin-kit-ai/sdk/internal/platforms/qoder",
			Status:          StatusRuntimeSupported,
			TransportModes:  []TransportMode{TransportProcess},
			LiveTestProfile: "qoder_plugin",
		},
		Launcher: LauncherMeta{Requirement: LauncherIgnored},
		NativeDocs: []NativeDocSpec{
			{Kind: "hooks", Path: authoredPath("targets/qoder/hooks/hooks.json"), Format: NativeDocJSON, Role: NativeDocRoleStructured},
		},
		SurfaceTiers: []SurfaceSupport{
			{Kind: "commands", Tier: SurfaceTierStable},
			{Kind: "agents", Tier: SurfaceTierUnsupported},
			{Kind: "hooks", Tier: SurfaceTierStable},
			{Kind: "output_styles", Tier: SurfaceTierUnsupported},
		},
		ManagedArtifacts: []ManagedArtifactSpec{
			{Kind: ManagedArtifactStatic, Path: ".qoder-plugin/plugin.json"},
			{Kind: ManagedArtifactPortableSkills, SourceRoot: authoredPath("skills"), OutputRoot: "skills"},
			{Kind: ManagedArtifactMirror, ComponentKind: "hooks", SourceRoot: authoredPath("targets/qoder/hooks"), OutputRoot: "hooks"},
			{Kind: ManagedArtifactMirror, ComponentKind: "commands", SourceRoot: authoredPath("targets/qoder/commands"), OutputRoot: "commands"},
			{Kind: ManagedArtifactPortableMCP, Path: ".mcp.json"},
		},
		Scaffold: qoderScaffold(),
		Validate: ValidateMeta{
			RequiredFiles: []string{
				"README.md",
				".qoder-plugin/plugin.json",
			},
			ForbiddenFiles: []string{
				"launcher.yaml",
			},
		},
	}
}

func qoderScaffold() ScaffoldMeta {
	return ScaffoldMeta{
		RequiredFiles: []string{
			authoredPath("plugin.yaml"),
			authoredPath("README.md"),
			"CLAUDE.md",
			"AGENTS.md",
		},
		OptionalFiles: []string{
			authoredPath("mcp/servers.yaml"),
			authoredPath("skills/{{.ProjectName}}/SKILL.md"),
			authoredPath("targets/qoder/hooks/hooks.json"),
			authoredPath("targets/qoder/commands"),
		},
		ForbiddenFiles: []string{
			"launcher.yaml",
		},
		TemplateFiles: []TemplateFile{
			{Path: authoredPath("plugin.yaml"), Template: "plugin.yaml.tmpl"},
			{Path: authoredPath("README.md"), Template: "qoder.README.md.tmpl"},
			{Path: authoredPath("mcp/servers.yaml"), Template: "mcp.servers.yaml.tmpl", Extra: true},
			{Path: authoredPath("skills/{{.ProjectName}}/SKILL.md"), Template: "SKILL.md.tmpl", Extra: true},
			{Path: authoredPath("targets/qoder/hooks/hooks.json"), Template: "targets.qoder.hooks.json.tmpl", Extra: true},
			{Path: authoredPath("targets/qoder/commands/{{.ProjectName}}.md"), Template: "targets.qoder.command.md.tmpl", Extra: true},
			{Path: "CLAUDE.md", Template: "ROOT.CLAUDE.md.tmpl"},
			{Path: "AGENTS.md", Template: "ROOT.AGENTS.md.tmpl"},
		},
	}
}
