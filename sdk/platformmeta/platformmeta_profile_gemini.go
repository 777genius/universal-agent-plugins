package platformmeta

func geminiProfile() PlatformProfile {
	return PlatformProfile{
		ID: "gemini",
		Contract: TargetContractMeta{
			PlatformFamily:         FamilyExtensionPackage,
			TargetClass:            "mcp_extension",
			TargetNoun:             "extension",
			ProductionClass:        "production-ready extension packaging lane",
			RuntimeContract:        "production-ready extension packaging plus optional production-ready 9-hook Go runtime and public-beta Notification observer",
			InstallModel:           "copy install",
			DevModel:               "link",
			ActivationModel:        "restart required",
			NativeRoot:             "~/.gemini/extensions/<name>",
			ImportSupport:          true,
			GenerateSupport:        true,
			ValidateSupport:        true,
			PortableComponentKinds: []string{"skills", "mcp_servers"},
			TargetComponentKinds:   []string{"package_metadata", "hooks", "commands", "policies", "themes", "settings", "contexts", "manifest_extra"},
			Summary:                "Gemini compiles as an official-style extension package with MCP, a primary root context, target-native extension assets, and an optional production-ready Go hook runtime lane.",
		},
		SDK: SDKMeta{
			PublicPackage:   "gemini",
			InternalPackage: "gemini",
			InternalImport:  "github.com/777genius/plugin-kit-ai/sdk/internal/platforms/gemini",
			Status:          StatusRuntimeSupported,
			TransportModes:  []TransportMode{TransportProcess},
			LiveTestProfile: "gemini_extension",
		},
		Launcher: LauncherMeta{Requirement: LauncherOptional},
		NativeDocs: []NativeDocSpec{
			{Kind: "package_metadata", Path: authoredPath("targets/gemini/package.yaml"), Format: NativeDocYAML, Role: NativeDocRoleStructured},
			{Kind: "manifest_extra", Path: authoredPath("targets/gemini/manifest.extra.json"), Format: NativeDocJSON, Role: NativeDocRoleExtra, ManagedKeys: []string{"name", "version", "description", "mcpServers", "contextFileName", "excludeTools", "migratedTo", "settings", "themes", "plan.directory"}},
		},
		SurfaceTiers: []SurfaceSupport{
			{Kind: "commands", Tier: SurfaceTierStable},
			{Kind: "hooks", Tier: SurfaceTierStable},
			{Kind: "policies", Tier: SurfaceTierStable},
			{Kind: "settings", Tier: SurfaceTierStable},
			{Kind: "themes", Tier: SurfaceTierStable},
			{Kind: "contexts", Tier: SurfaceTierStable},
			{Kind: "manifest_extra", Tier: SurfaceTierStable},
			{Kind: "agents", Tier: SurfaceTierPreview},
		},
		ManagedArtifacts: []ManagedArtifactSpec{
			{Kind: ManagedArtifactStatic, Path: "gemini-extension.json"},
			{Kind: ManagedArtifactPortableSkills, SourceRoot: authoredPath("skills"), OutputRoot: "skills"},
			{Kind: ManagedArtifactMirror, ComponentKind: "hooks", SourceRoot: authoredPath("targets/gemini/hooks"), OutputRoot: "hooks"},
			{Kind: ManagedArtifactMirror, ComponentKind: "commands", SourceRoot: authoredPath("targets/gemini/commands"), OutputRoot: "commands"},
			{Kind: ManagedArtifactMirror, ComponentKind: "policies", SourceRoot: authoredPath("targets/gemini/policies"), OutputRoot: "policies"},
			{Kind: ManagedArtifactMirror, ComponentKind: "contexts", SourceRoot: authoredPath("targets/gemini/contexts"), OutputRoot: "contexts"},
			{Kind: ManagedArtifactSelectedContext, ComponentKind: "contexts", OutputRoot: "", ContextMode: ContextStrategyGeminiPrimaryRoot},
		},
		Scaffold: ScaffoldMeta{
			RequiredFiles: []string{
				authoredPath("plugin.yaml"),
				authoredPath("targets/gemini/package.yaml"),
				authoredPath("targets/gemini/contexts/GEMINI.md"),
				authoredPath("README.md"),
				"CLAUDE.md",
				"AGENTS.md",
			},
			OptionalFiles: []string{
				"Makefile",
				".goreleaser.yml",
				authoredPath("skills/{{.ProjectName}}/SKILL.md"),
			},
			TemplateFiles: []TemplateFile{
				{Path: authoredPath("plugin.yaml"), Template: "plugin.yaml.tmpl"},
				{Path: authoredPath("targets/gemini/package.yaml"), Template: "targets.gemini.package.yaml.tmpl"},
				{Path: authoredPath("targets/gemini/contexts/GEMINI.md"), Template: "gemini.GEMINI.md.tmpl"},
				{Path: authoredPath("README.md"), Template: "gemini.README.md.tmpl"},
				{Path: "CLAUDE.md", Template: "ROOT.CLAUDE.md.tmpl"},
				{Path: "AGENTS.md", Template: "ROOT.AGENTS.md.tmpl"},
				{Path: "Makefile", Template: "Makefile.tmpl", Extra: true},
				{Path: ".goreleaser.yml", Template: "goreleaser.yml.tmpl", Extra: true},
				{Path: authoredPath("skills/{{.ProjectName}}/SKILL.md"), Template: "SKILL.md.tmpl", Extra: true},
			},
		},
		Validate: ValidateMeta{
			RequiredFiles: []string{
				"README.md",
				"gemini-extension.json",
			},
		},
	}
}
