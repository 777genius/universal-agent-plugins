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
	if !ok || gemini.Traits.LifecycleKind != LifecycleNativeConfig {
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
