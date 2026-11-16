package domain

import "testing"

func TestGrokManagedStdioIsProjectOnlyAndRemainsPrepared(t *testing.T) {
	selected := SelectedDelivery{}
	user := selected.EffectiveTraitsForScope(ClientGrok, ScopeUser)
	project := selected.EffectiveTraitsForScope(ClientGrok, ScopeProject)
	if user.UsesManagedStdioLauncher || user.RequiresActiveNativeProjection {
		t.Fatal("Grok User acquired Project policy", user)
	}
	if !project.UsesManagedStdioLauncher || !project.RequiresActiveNativeProjection || !project.TracksNativeEffects || project.LifecycleKind != LifecyclePrepared {
		t.Fatal("Grok Project policy changed lifecycle", project)
	}
	for _, id := range []ClientID{ClientAntigravity, ClientQoder} {
		traits := selected.EffectiveTraitsForScope(id, ScopeProject)
		if traits.RequiresActiveNativeProjection {
			t.Fatal("other prepared client acquired Grok readback", id)
		}
	}
}
