package domain

import "testing"

func TestClientTraitsAreDeclarativeAndDrivePolicy(t *testing.T) {
	codex, ok := ClientDefinitionFor(ClientCodex)
	if !ok || !codex.Traits.HonorsOpenAIMCPAuthHints || !codex.Traits.SupportsPreparedRecovery || codex.Traits.LifecycleKind != LifecycleCLIRegistry {
		t.Fatalf("Codex traits = %+v", codex.Traits)
	}
	if codex.Capabilities.MCPTransports["sse"] != SupportUnsupported || codex.Capabilities.AppSupport != SupportUnsupported {
		t.Fatalf("Codex capabilities drifted: %+v", codex.Capabilities)
	}
	chatgpt, ok := ClientDefinitionFor(ClientChatGPT)
	if !ok || !chatgpt.Traits.RequiresPersonalMappingForPrepare || chatgpt.Traits.LifecycleKind != LifecycleManual || !chatgpt.Traits.Allows(InstallIntentPrepare) {
		t.Fatalf("ChatGPT traits = %+v", chatgpt.Traits)
	}
	if chatgpt.Capabilities.AppSupport != SupportProjected {
		t.Fatalf("ChatGPT app support = %s, want projected", chatgpt.Capabilities.AppSupport)
	}
	kiro, ok := ClientDefinitionFor(ClientKiro)
	if !ok || !kiro.Traits.Allows(InstallIntentPrepare) || kiro.Traits.RequiresPersonalMappingForPrepare {
		t.Fatalf("Kiro traits = %+v", kiro.Traits)
	}
	if err := InstallIntentPrepare.Validate(ClientKiro); err != nil {
		t.Fatal(err)
	}
	if err := InstallIntentPrepare.Validate(ClientChatGPT); err != nil {
		t.Fatal(err)
	}
	if err := InstallIntentPrepare.Validate(ClientCodex); err == nil {
		t.Fatal("Codex must reject prepare")
	}
	if err := InstallIntentAutomatic.Validate(ClientID("unknown")); err != nil {
		t.Fatal(err)
	}
	claude, ok := ClientDefinitionFor(ClientClaude)
	if !ok || !claude.Traits.UsesManagedStdioLauncher || claude.Traits.LifecycleKind != LifecycleCLIRegistry {
		t.Fatalf("Claude traits = %+v", claude.Traits)
	}
	windsurf, ok := ClientDefinitionFor(ClientWindsurf)
	if !ok || !windsurf.Traits.UsesManagedStdioLauncher || windsurf.Traits.LifecycleKind != LifecycleNativeConfig {
		t.Fatalf("Windsurf traits = %+v", windsurf.Traits)
	}
	gemini, ok := ClientDefinitionFor(ClientGemini)
	if !ok || gemini.Traits.LifecycleKind != LifecycleNativeConfig || !gemini.Traits.BindsNativeProfileRoot {
		t.Fatalf("Gemini traits = %+v", gemini.Traits)
	}
	opencode, ok := ClientDefinitionFor(ClientOpenCode)
	if !ok || !opencode.Traits.ReportsMCPToolNamespaceCollision || opencode.Traits.LifecycleKind != LifecycleNativeConfig {
		t.Fatalf("OpenCode traits = %+v", opencode.Traits)
	}
}

func TestShouldReadOnlyVerifyFollowsTraitsNotClientIDs(t *testing.T) {
	if !ShouldReadOnlyVerify(ClientGemini, "", InstallIntentAutomatic) {
		t.Fatal("native-config clients verify without an executable")
	}
	if ShouldReadOnlyVerify(ClientClaude, "", InstallIntentAutomatic) {
		t.Fatal("Claude skips verify when the host CLI is absent")
	}
	if !ShouldReadOnlyVerify(ClientClaude, "/bin/claude", InstallIntentAutomatic) {
		t.Fatal("Claude verifies when the host CLI is present")
	}
	if ShouldReadOnlyVerify(ClientCodex, "", InstallIntentAutomatic) {
		t.Fatal("Codex skips verify when the host CLI is absent")
	}
	if !ShouldReadOnlyVerify(ClientCodex, "/bin/codex", InstallIntentAutomatic) {
		t.Fatal("Codex verifies when the host CLI is present")
	}
	if ShouldReadOnlyVerify(ClientCopilot, "", InstallIntentAutomatic) {
		t.Fatal("shared-backend clients skip verify when the host CLI is absent")
	}
	if !ShouldReadOnlyVerify(ClientVSCode, "/bin/copilot", InstallIntentAutomatic) {
		t.Fatal("shared-backend clients verify when the host CLI is present")
	}
	if ShouldReadOnlyVerify(ClientCursor, "/bin/cursor", InstallIntentAutomatic) {
		t.Fatal("Cursor has no read-only verifier")
	}
	if ShouldReadOnlyVerify(ClientChatGPT, "", InstallIntentPrepare) {
		t.Fatal("ChatGPT prepare does not run read-only verify")
	}
	if !ShouldReadOnlyVerify(ClientKiro, "", InstallIntentPrepare) {
		t.Fatal("Kiro prepare verifies without a host CLI")
	}
	if ShouldReadOnlyVerify(ClientKiro, "/bin/other", InstallIntentAutomatic) {
		t.Fatal("Kiro automatic skips verify when the executable does not name the client")
	}
	if !ShouldReadOnlyVerify(ClientKiro, "/bin/kiro", InstallIntentAutomatic) {
		t.Fatal("Kiro automatic verifies when the executable names the client")
	}
}

func TestRequiresNativeProjectorMatchesPackageAndLifecycle(t *testing.T) {
	if !RequiresNativeProjector(ClientGemini) || !RequiresNativeProjector(ClientOpenCode) || !RequiresNativeProjector(ClientVSCode) || !RequiresNativeProjector(ClientKiro) {
		t.Fatal("native and native-config clients require a projector")
	}
	if RequiresNativeProjector(ClientClaude) || RequiresNativeProjector(ClientCodex) || RequiresNativeProjector(ClientChatGPT) {
		t.Fatal("projection and manual clients do not require a projector")
	}
}

// Siblings install one shared package through one backend, so they must
// agree on every command they receive. Otherwise one of them withholding a
// command, an explicit-only one in particular, while the other delivers it
// would put the command in front of the first client after all, and each
// update would rewrite the shared package for its own client.
func TestBackendSiblingsAgreeOnCommandDelivery(t *testing.T) {
	for _, definition := range ClientDefinitions() {
		for _, id := range BackendSiblings(definition.ID) {
			sibling, _ := ClientDefinitionFor(id)
			if sibling.Traits.EnforcesExplicitSkillInvocation != definition.Traits.EnforcesExplicitSkillInvocation {
				t.Errorf("%s and its backend sibling %s disagree on EnforcesExplicitSkillInvocation", definition.ID, id)
			}
			if sibling.CommandSupport != definition.CommandSupport {
				t.Errorf("%s and its backend sibling %s disagree on CommandSupport", definition.ID, id)
			}
		}
	}
}

// Visual Studio Code is frozen, so the github-copilot family it shares a
// package with receives no commands; every other client takes them as skills.
func TestOnlyTheFrozenFamilyWithholdsCommands(t *testing.T) {
	for _, definition := range ClientDefinitions() {
		withheld := definition.CommandSupport == SupportUnsupported
		if family := definition.BackendFamily == "github-copilot" || definition.ID == ClientPi; withheld != family {
			t.Errorf("%s withholds commands = %v, want %v", definition.ID, withheld, family)
		}
		if DeliversCommand(Command{Name: "echo"}, definition) == withheld {
			t.Errorf("%s delivers an ordinary command although it withholds commands = %v", definition.ID, withheld)
		}
	}
}

// Global and project Pi both support native Skills/MCP. Only project Pi
// tracks native attempts; user Pi publishes through its installed host callback.
// Tracking user Pi before that callback blocks installed authority publication.
func TestPiNativeEffectTrackingIsProjectScoped(t *testing.T) {
	user := (SelectedDelivery{}).EffectiveTraitsForScope(ClientPi, ScopeUser)
	project := (SelectedDelivery{}).EffectiveTraitsForScope(ClientPi, ScopeProject)
	if user.TracksNativeEffects || user.LifecycleKind != LifecyclePrepared {
		t.Fatalf("Pi user lifecycle unexpectedly requires a native attempt: %+v", user)
	}
	if !user.RequiresActiveNativeProjection {
		t.Fatal("Pi user lost read-only native projection for global Skills/MCP after the host's commit handoff")
	}
	if !project.TracksNativeEffects || project.LifecycleKind != LifecyclePrepared {
		t.Fatalf("Pi project lifecycle lost tracked native effects: %+v", project)
	}
	pi, ok := ClientDefinitionFor(ClientPi)
	if !ok {
		t.Fatal("Pi client definition missing")
	}
	userCaps, projectCaps := pi.ScopedCapabilities(ScopeUser), pi.ScopedCapabilities(ScopeProject)
	if userCaps.SkillSupport != SupportNative || userCaps.MCPTransports["stdio"] != SupportNative ||
		userCaps.MCPTransports["streamable-http"] != SupportNative {
		t.Fatalf("Pi global Skills/MCP contract regressed: %+v", userCaps)
	}
	if projectCaps.SkillSupport != SupportNative || projectCaps.MCPTransports["stdio"] != SupportNative ||
		projectCaps.MCPTransports["streamable-http"] != SupportNative {
		t.Fatalf("Pi project lost its selected components: %+v", projectCaps)
	}
}

func TestAntigravityPreparedScopesKeepFileOnlyVerificationRouting(t *testing.T) {
	for _, scope := range []InstallScope{ScopeUser, ScopeProject} {
		traits := (SelectedDelivery{}).EffectiveTraitsForScope(ClientAntigravity, scope)
		if traits.LifecycleKind != LifecyclePrepared || traits.TracksNativeEffects || traits.VerifiesNativeDirectory {
			t.Fatalf("Antigravity scope %s changed lifecycle effects: %+v", scope, traits)
		}
		for _, backend := range []string{"", "/selected/agy"} {
			if ShouldReadOnlyVerifyForScope(ClientAntigravity, scope, backend, InstallIntentAutomatic) {
				t.Fatal("Prepared ownership entered generic Active-only verification", scope, backend)
			}
		}
	}
}
