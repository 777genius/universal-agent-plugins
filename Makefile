.PHONY: test test-required test-agentplugins-native-install test-plugin-manifest-workflow test-install-compat test-extended test-polyglot-smoke test-live test-live-cli test-install-live test-gemini-live test-gemini-runtime test-gemini-runtime-live test-opencode-live test-opencode-cli-live test-opencode-tools-live test-opencode-mcp-live test-opencode-e2e-live test-cursor-live test-portable-mcp-live test-context7-live test-chrome-devtools-live test-atlassian-live test-cloudflare-live test-cloudflare-bindings-live test-cloudflare-docs-live test-cloudflare-observability-live test-cloudflare-radar-live test-heroku-live test-hubspot-crm-live test-hubspot-developer-live test-neon-live test-docker-hub-live test-notion-live test-e2e-live test-govulncheck-local test-security generated-check version-sync-check removed-contract-boundary-check release-gate release-rehearsal build-plugin-kit-ai vet

GOCACHE ?= /tmp/plugin-kit-ai-gocache
export GOCACHE

SECURITY_GOTOOLCHAIN ?= go1.25.13

EXTENDED_TEST_ARGS ?=
REQUIRED_TEST_TIMEOUT ?= 20m

test:
	$(MAKE) test-required

test-required:
	go test -count=1 -timeout=$(REQUIRED_TEST_TIMEOUT) ./...
	go test -count=1 -timeout=$(REQUIRED_TEST_TIMEOUT) ./cli/plugin-kit-ai/...
	go test -count=1 -timeout=$(REQUIRED_TEST_TIMEOUT) ./install/integrationctl/...
	go test -count=1 -timeout=$(REQUIRED_TEST_TIMEOUT) ./install/plugininstall/...
	go test -count=1 -timeout=$(REQUIRED_TEST_TIMEOUT) ./sdk/...
	cd npm/agentplugins && npm test && npm pack --dry-run --ignore-scripts

test-agentplugins-native-install:
	go test -count=1 -run '^TestAgentpluginsNativeInstaller' ./repotests

test-plugin-manifest-workflow:
	go test -count=1 -run 'TestPluginKitAI(ValidateWarnsButSucceedsOnExtraPluginYAMLFields|ValidateStrictFailsOnWarningsThenNormalizeFixesThem|ImportPrintsWarningsForIgnoredAssets|MigrationFixtures_RoundTripToStrictValidation)$$' ./repotests

test-install-compat:
	go test -count=1 -run '^TestPluginKitAIInstall_' ./repotests

test-extended:
	go test -count=1 -run '^TestClaudeCLIHooks$$' ./repotests $(EXTENDED_TEST_ARGS)
	go test -count=1 -run '^TestCodexCLINotify$$' ./repotests $(EXTENDED_TEST_ARGS)
	go test -count=1 -run '^TestOpenCodeCLIPluginLoadSmoke$$' ./repotests $(EXTENDED_TEST_ARGS)

test-polyglot-smoke:
	go test -count=1 -run 'TestRenderTemplate_(PythonLauncherWindowsFallbackOrder|ShellLauncherWindowsRequiresBash)$$' ./cli/plugin-kit-ai/internal/scaffold
	go test -count=1 -run 'Test(FindPython_UsesPlatformAwareLookupOrder|Validate_ManifestProject_WindowsCmdLauncherAccepted|Validate_ManifestProject_ShellRequiresBashOnWindows|ValidateNodeRuntimeTarget_MissingBuiltOutputShowsRecoveryGuidance|ValidateRuntimeTargetExecutable_NonExecutableScriptFails|ShellLauncherPassthrough)$$' ./cli/plugin-kit-ai/internal/validate
	go test -count=1 -run 'TestPluginService(DoctorReadyNeedsBootstrapNeedsBuildAndBlocked|DoctorPoetryManagerOwnedEnvIsReady|BootstrapPythonCreatesVenvAndInstallsRequirements|BootstrapPoetryReportsManagerOwnedEnv|BootstrapNodePNPMTypeScriptRunsInstallAndBuild|ExportPythonBundleExcludesProjectVenv|ExportShellBundlePreservesScripts|ExportRejectsGoRuntime|BundleInstallInstallsPythonBundleIntoDestination|BundleInstallRejectsRemoteURL|BundleFetchURLInstallsPythonBundleWithExplicitChecksum|BundleFetchURLUsesSidecarChecksum|BundleFetchURLRejectsHTTP|BundleFetchURLFailsChecksumMismatch|BundleFetchGitHubInstallsNodeBundleFromChecksumsTxt|BundleFetchGitHubFallsBackToSidecarChecksum|BundleFetchGitHubUsesLatestRelease|BundleFetchGitHubRejectsMetadataMismatch|BundlePublishCreatesPublishedReleaseByDefault|BundlePublishCreatesDraftReleaseWhenRequested|BundlePublishPromotesExistingDraftReleaseToPublished|BundlePublishReusesExistingDraftReleaseWhenRequested|BundlePublishReusesExistingPublishedReleaseWithForce|BundlePublishFailsWhenAssetExistsWithoutForce|BundlePublishRejectsShellRuntime)$$|TestSelectBundleReleaseAsset(RejectsAmbiguous|UsesPlatformRuntime|UsesExactAssetName)$$' ./cli/plugin-kit-ai/internal/app
	go test -count=1 -run 'TestBundle(Install(HelpIncludesLocalTarballLanguage|WritesRunnerOutput)|Fetch(HelpIncludesURLAndGitHubLanguage|WritesRunnerOutput)|Publish(HelpIncludesGitHubLanguage|WritesRunnerOutput))$$' ./cli/plugin-kit-ai/cmd/plugin-kit-ai
	go test -count=1 -run 'TestPluginKitAI(Init(GoRuntimeLauncherFlow|PythonRuntimeLauncherFlow|PythonRuntimeWithRequirementsDoctorBootstrapFlow|PythonRuntimeBrokenVenvFailsValidate|ShellRuntimeLauncherFlow|ShellRuntimeNonExecutableTargetFailsValidate|NodeRuntimeSupportsTypeScriptBuildThroughLauncher|NodeRuntimePNPMDoctorBootstrapFlow|NodeRuntimeMissingBuiltOutputFailsValidate)|RuntimeABIPassthrough|PythonLauncherPrefersProjectVenvOnWindows)$$' ./repotests
	go test -count=1 -run 'TestPluginKitAI(BootstrapScriptInstallsLatestRelease|BootstrapScriptSupportsExplicitVersion|BootstrapScriptRejectsChecksumMismatch|InitExtras(PythonEmitsBundleReleaseWorkflow|NodeTypeScriptEmitsBundleReleaseWorkflow))$$|TestSetupPluginKitAIActionUsesInstallScript$$' ./repotests
	go test -count=1 -run 'Test(HomebrewFormulaGeneratorFromChecksums|NPMCLIPackageContractFiles|PythonCLIPackageContractFiles|NPMRuntimePackage(ContractFiles|ClaudeAndCodexSmoke)|PythonRuntimePackage(ContractFiles|ClaudeAndCodexSmoke)|StarterRepos_(LayoutAndReadmesStayAligned|Smoke)|StarterTemplate(SyncContractFilesStayAligned|SyncScriptSupportsLocalMirror|RepoLinksResolveToCurrentOwnerNaming)|ReleaseSurface_MakefileDocsAndWorkflowsStayAligned|ContractClarity_RuntimeMetadataAndDocsStayAligned)$$' ./repotests
	go test -count=1 -run 'TestPluginKitAIExport(PythonRequirementsBundleFlow|NodeTypeScriptBundleFlow|ShellBundleFlow)|TestPluginKitAIBundleInstall(PythonRequirementsFlow|NodeTypeScriptFlow|ClaudeNodeTypeScriptFlow)$$' ./repotests
	go test -count=1 -run 'TestPluginKitAIBundle(Fetch(URL(PythonRequirementsFlow|ClaudeNodeTypeScriptFlow)|GitHub(ClaudeNodeTypeScriptFlow|LatestClaudeNodeTypeScriptFlow))|PublishFetch(PythonRequirementsFlow|ClaudeNodeTypeScriptFlow))$$' ./repotests
	go test -count=1 -run '^TestNPMCLIPackage' ./repotests
	go test -count=1 -run '^TestPythonCLIPackage' ./repotests

# Live E2E: real GitHub + real claude-notifications-go release (needs network). Optional: GITHUB_TOKEN.
# Package is ./repotests (tests moved out of repo root).
test-live: test-e2e-live

test-live-cli:
	go test -count=1 -run 'TestClaudeHooks_LiveHaikuLow' ./repotests $(EXTENDED_TEST_ARGS)
	go test -count=1 -run '^TestCodexCLINotify$$' ./repotests $(EXTENDED_TEST_ARGS)
	go test -count=1 -run '^TestOpenCodeCLIPluginLoadSmoke$$' ./repotests $(EXTENDED_TEST_ARGS)
	go test -count=1 -run '^TestCursorCLI' ./repotests $(EXTENDED_TEST_ARGS)

test-install-live:
	PLUGIN_KIT_AI_E2E_LIVE=1 go test -count=1 -timeout=15m -run '^TestLiveInstall_' ./repotests

test-gemini-live:
	PLUGIN_KIT_AI_RUN_GEMINI_CLI=1 go test -count=1 -run '^TestGeminiCLIExtensionLink$$' ./repotests $(EXTENDED_TEST_ARGS)

test-gemini-runtime:
	go test -count=1 ./sdk/... $(EXTENDED_TEST_ARGS)
	go test -count=1 -run 'TestInitRunner_geminiGoRuntimeStarter' ./cli/plugin-kit-ai/internal/app $(EXTENDED_TEST_ARGS)
	go test -count=1 -run 'TestInspectTextShowsLauncherAndGeminiGuidance' ./cli/plugin-kit-ai/cmd/plugin-kit-ai $(EXTENDED_TEST_ARGS)
	go test -count=1 -run 'TestPluginKitAIInitGeminiGoRuntimeLauncherFlow|TestGeneratedConfigCanaries_GeminiRuntimeContract|TestGeminiE2ETracePreservesOriginalRequestName|TestGeminiE2ETraceCapturesModelAndToolSelectionPayloads|TestGeminiE2ETraceCapturesRuntimeLifecycleHooks|TestGeminiE2ETraceCapturesRuntimeControlSemantics|TestGeminiE2ETraceCapturesRuntimeTransformSemantics|TestContractClarity_GeminiRuntimeDocsStayAligned' ./repotests $(EXTENDED_TEST_ARGS)

test-gemini-runtime-live:
	PLUGIN_KIT_AI_RUN_GEMINI_RUNTIME_LIVE=1 go test -count=1 -run '^TestGeminiCLIRuntime(Hooks|BeforeToolDeny|BeforeModelDeny|DisableAllTools|AfterModelReplaceResponse|AfterAgentRetry|RewriteToolInput)$$' ./repotests $(EXTENDED_TEST_ARGS)

test-opencode-live:
	PLUGIN_KIT_AI_ENABLE_OPENCODE_SMOKE=1 go test -count=1 -run '^TestOpenCodeLoaderSmoke$$' ./repotests $(EXTENDED_TEST_ARGS)

test-opencode-cli-live:
	PLUGIN_KIT_AI_RUN_OPENCODE_CLI=1 go test -count=1 -run '^TestOpenCodeCLIPluginLoadSmoke$$' ./repotests $(EXTENDED_TEST_ARGS)

test-opencode-tools-live:
	PLUGIN_KIT_AI_ENABLE_OPENCODE_SMOKE=1 go test -count=1 -run '^TestOpenCodeStandaloneToolsSmoke$$' ./repotests $(EXTENDED_TEST_ARGS)

test-opencode-mcp-live:
	PLUGIN_KIT_AI_RUN_PORTABLE_MCP_LIVE=1 go test -count=1 -run 'TestPortableMCPLiveAcrossConsoleAgents/OpenCode_loader_initializes_shared_MCP' ./repotests $(EXTENDED_TEST_ARGS)

test-opencode-e2e-live:
	$(MAKE) test-opencode-live EXTENDED_TEST_ARGS='$(EXTENDED_TEST_ARGS)'
	$(MAKE) test-opencode-cli-live EXTENDED_TEST_ARGS='$(EXTENDED_TEST_ARGS)'
	$(MAKE) test-opencode-mcp-live EXTENDED_TEST_ARGS='$(EXTENDED_TEST_ARGS)'

test-cursor-live:
	PLUGIN_KIT_AI_RUN_CURSOR_CLI=1 go test -count=1 -run '^TestCursorCLI' ./repotests $(EXTENDED_TEST_ARGS)

test-portable-mcp-live:
	PLUGIN_KIT_AI_RUN_PORTABLE_MCP_LIVE=1 go test -count=1 -run '^TestPortableMCPLiveAcrossConsoleAgents$$' ./repotests $(EXTENDED_TEST_ARGS)

test-context7-live:
	PLUGIN_KIT_AI_RUN_CONTEXT7_LIVE=1 go test -count=1 -run '^TestContext7CatalogLiveAcrossInstalledAgents$$' ./repotests $(EXTENDED_TEST_ARGS)

test-chrome-devtools-live:
	PLUGIN_KIT_AI_RUN_CHROME_DEVTOOLS_LIVE=1 go test -count=1 -run '^TestChromeDevtoolsCatalogLiveAcrossInstalledAgents$$' ./repotests $(EXTENDED_TEST_ARGS)

test-atlassian-live:
	PLUGIN_KIT_AI_RUN_ATLASSIAN_LIVE=1 go test -count=1 -run '^TestAtlassianCatalogLiveAcrossInstalledAgents$$' ./repotests $(EXTENDED_TEST_ARGS)

test-cloudflare-live:
	PLUGIN_KIT_AI_RUN_CLOUDFLARE_LIVE=1 go test -count=1 -run '^TestCloudflareCatalogLiveAcrossInstalledAgents$$' ./repotests $(EXTENDED_TEST_ARGS)

test-cloudflare-bindings-live:
	PLUGIN_KIT_AI_RUN_CLOUDFLARE_BINDINGS_LIVE=1 go test -count=1 -run '^TestCloudflareBindingsCatalogLiveAcrossInstalledAgents$$' ./repotests $(EXTENDED_TEST_ARGS)

test-cloudflare-docs-live:
	PLUGIN_KIT_AI_RUN_CLOUDFLARE_DOCS_LIVE=1 go test -count=1 -run '^TestCloudflareDocsCatalogLiveAcrossInstalledAgents$$' ./repotests $(EXTENDED_TEST_ARGS)

test-cloudflare-observability-live:
	PLUGIN_KIT_AI_RUN_CLOUDFLARE_OBSERVABILITY_LIVE=1 go test -count=1 -run '^TestCloudflareObservabilityCatalogLiveAcrossInstalledAgents$$' ./repotests $(EXTENDED_TEST_ARGS)

test-cloudflare-radar-live:
	PLUGIN_KIT_AI_RUN_CLOUDFLARE_RADAR_LIVE=1 go test -count=1 -run '^TestCloudflareRadarCatalogLiveAcrossInstalledAgents$$' ./repotests $(EXTENDED_TEST_ARGS)

test-heroku-live:
	PLUGIN_KIT_AI_RUN_HEROKU_LIVE=1 go test -count=1 -run '^TestHerokuCatalogLiveAcrossInstalledAgents$$' ./repotests $(EXTENDED_TEST_ARGS)

test-hubspot-crm-live:
	PLUGIN_KIT_AI_RUN_HUBSPOT_CRM_LIVE=1 go test -count=1 -run '^TestHubSpotCRMCatalogLiveAcrossInstalledAgents$$' ./repotests $(EXTENDED_TEST_ARGS)

test-hubspot-developer-live:
	PLUGIN_KIT_AI_RUN_HUBSPOT_DEVELOPER_LIVE=1 go test -count=1 -run '^TestHubSpotDeveloperCatalogLiveAcrossInstalledAgents$$' ./repotests $(EXTENDED_TEST_ARGS)

test-neon-live:
	PLUGIN_KIT_AI_RUN_NEON_LIVE=1 go test -count=1 -run '^TestNeonCatalogLiveAcrossInstalledAgents$$' ./repotests $(EXTENDED_TEST_ARGS)

test-docker-hub-live:
	PLUGIN_KIT_AI_RUN_DOCKER_HUB_LIVE=1 go test -count=1 -run '^TestDockerHubCatalogLiveAcrossInstalledAgents$$' ./repotests $(EXTENDED_TEST_ARGS)

test-notion-live:
	PLUGIN_KIT_AI_RUN_NOTION_LIVE=1 go test -count=1 -run '^TestNotionCatalogLiveAcrossInstalledAgents$$' ./repotests $(EXTENDED_TEST_ARGS)

test-vercel-live:
	PLUGIN_KIT_AI_RUN_VERCEL_LIVE=1 go test -count=1 -run '^TestVercelCatalogLiveAcrossInstalledAgents$$' ./repotests $(EXTENDED_TEST_ARGS)

test-sentry-live:
	PLUGIN_KIT_AI_RUN_SENTRY_LIVE=1 go test -count=1 -run '^TestSentryCatalogLiveAcrossInstalledAgents$$' ./repotests $(EXTENDED_TEST_ARGS)

test-stripe-live:
	PLUGIN_KIT_AI_RUN_STRIPE_LIVE=1 go test -count=1 -run '^TestStripeCatalogLiveAcrossInstalledAgents$$' ./repotests $(EXTENDED_TEST_ARGS)

test-slack-live:
	PLUGIN_KIT_AI_RUN_SLACK_LIVE=1 go test -count=1 -run '^TestSlackCatalogLiveAcrossSupportedAgents$$' ./repotests $(EXTENDED_TEST_ARGS)

test-e2e-live: test-install-live

test-govulncheck-local:
	GOTOOLCHAIN=$(SECURITY_GOTOOLCHAIN) go run golang.org/x/vuln/cmd/govulncheck@v1.6.0 ./...
	cd cli/plugin-kit-ai && GOTOOLCHAIN=$(SECURITY_GOTOOLCHAIN) go run golang.org/x/vuln/cmd/govulncheck@v1.6.0 ./...
	cd install/plugininstall && GOTOOLCHAIN=$(SECURITY_GOTOOLCHAIN) go run golang.org/x/vuln/cmd/govulncheck@v1.6.0 ./...
	cd install/integrationctl && GOTOOLCHAIN=$(SECURITY_GOTOOLCHAIN) go run golang.org/x/vuln/cmd/govulncheck@v1.6.0 ./...
	cd sdk && GOTOOLCHAIN=$(SECURITY_GOTOOLCHAIN) go run golang.org/x/vuln/cmd/govulncheck@v1.6.0 ./...

test-security: test-govulncheck-local

# Root module is workspace-only; submodules are vetted explicitly.
vet:
	go vet ./...
	cd cli/plugin-kit-ai && go vet ./...
	cd install/integrationctl && go vet ./...
	cd install/plugininstall && go vet ./...
	cd sdk && go vet ./...

generated-check:
	bash ./scripts/check-generated-sync.sh
	$(MAKE) version-sync-check
	$(MAKE) removed-contract-boundary-check

version-sync-check:
	bash ./scripts/check-version-sync.sh

removed-contract-boundary-check:
	bash ./scripts/check-removed-contract-boundary.sh

release-gate:
	$(MAKE) test-required
	$(MAKE) vet
	$(MAKE) generated-check

release-rehearsal: release-gate
	$(MAKE) test-install-compat
	$(MAKE) test-polyglot-smoke
	$(MAKE) test-security
	@echo "Release rehearsal deterministic checks complete. Record dependency-review, CodeQL, extended/live evidence (including OpenCode smoke when refreshing that stable boundary), audit updates, release notes draft, artifact attestations, and any waiver notes tied to the candidate commit SHA."

build-plugin-kit-ai:
	go build -o bin/plugin-kit-ai ./cli/plugin-kit-ai/cmd/plugin-kit-ai
