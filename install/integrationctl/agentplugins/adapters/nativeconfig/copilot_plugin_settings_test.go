package nativeconfig

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

func TestCopilotPluginSettingsAddRepeatRemovePreservesForeignJSONC(t *testing.T) {
	reg := CopilotPluginRegistration{Marketplace: "agentplugins-abc123", Plugin: "review", Directory: ".uap/managed/clients/copilot/fixture"}
	foreign := []byte("// keep comments\n{\"other\":9007199254740993,\"extraKnownMarketplaces\":{\"foreign\":{\"source\":{\"source\":\"github\",\"repo\":\"keep\"}}},\"enabledPlugins\":{\"foreign@foreign\":false},}\n")
	_, exact, err := InspectCopilotPluginSettings(foreign, reg)
	if err != nil || exact {
		t.Fatalf("fresh namespace: exact=%v err=%v", exact, err)
	}
	added, changed, err := PatchCopilotPluginSettings(foreign, reg, false, false)
	if err != nil || !changed || !bytes.Contains(added, []byte("9007199254740993")) || !bytes.Contains(added, []byte("// keep comments")) {
		t.Fatalf("add did not preserve foreign JSONC: %v\n%s", err, added)
	}
	present, exact, err := InspectCopilotPluginSettings(added, reg)
	if err != nil || !present || !exact {
		t.Fatalf("added registration unrecognized: %v %v %v", present, exact, err)
	}
	repeat, changed, err := PatchCopilotPluginSettings(added, reg, true, false)
	if err != nil || changed || !bytes.Equal(added, repeat) {
		t.Fatalf("repeat rewrote settings: %v", err)
	}
	removed, changed, err := PatchCopilotPluginSettings(added, reg, true, true)
	if err != nil || !changed || !bytes.Contains(removed, []byte("foreign@foreign")) || !bytes.Contains(removed, []byte("9007199254740993")) {
		t.Fatalf("remove damaged foreign settings: %v\n%s", err, removed)
	}
	present, _, err = InspectCopilotPluginSettings(removed, reg)
	if err != nil || present {
		t.Fatalf("remove left registration: %v %v", present, err)
	}
}

func TestCopilotPluginSettingsRefusesUnownedAndChangedEntries(t *testing.T) {
	reg := CopilotPluginRegistration{Marketplace: "agentplugins-abc123", Plugin: "review", Directory: ".uap/managed/clients/copilot/fixture"}
	for _, body := range []string{
		"{\"extraKnownMarketplaces\":{\"agentplugins-abc123\":{\"source\":{\"source\":\"directory\",\"path\":\"other\"}}}}",
		"{\"enabledPlugins\":{\"review@agentplugins-abc123\":true}}",
		"{\"extraKnownMarketplaces\":{\"agentplugins-abc123\":{\"source\":{\"source\":\"directory\",\"path\":\".uap/managed/clients/copilot/fixture\"}}},\"enabledPlugins\":{\"review@agentplugins-abc123\":false}}",
	} {
		for _, owned := range []bool{false, true} {
			_, _, err := PatchCopilotPluginSettings([]byte(body), reg, owned, false)
			if !errors.Is(err, ErrCollision) && !errors.Is(err, ErrNotOwned) {
				t.Fatalf("collision/drift escaped (owned=%v): %v", owned, err)
			}
			_, _, err = PatchCopilotPluginSettings([]byte(body), reg, owned, true)
			if owned && !errors.Is(err, ErrNotOwned) {
				t.Fatalf("remove mutated drift: %v", err)
			}
		}
	}
	added, _, err := PatchCopilotPluginSettings(nil, reg, false, false)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := PatchCopilotPluginSettings(added, reg, false, false); !errors.Is(err, ErrCollision) {
		t.Fatal("identical unowned registration silently adopted")
	}
}

func TestCopilotPluginSettingsRejectsInvalidAndAmbiguousDocuments(t *testing.T) {
	reg := CopilotPluginRegistration{Marketplace: "agentplugins-abc123", Plugin: "review", Directory: ".uap/managed/clients/copilot/fixture"}
	for _, body := range []string{
		"[]", "{", "{\"enabledPlugins\":true}",
		"{\"enabledPlugins\":{},\"enabledPlugins\":{}}",
		"{\"other\":{\"a\":1,\"a\":2}}",
	} {
		if _, _, err := PatchCopilotPluginSettings([]byte(body), reg, false, false); err == nil {
			t.Fatalf("malformed native settings accepted: %s", body)
		}
	}
	for _, bad := range []string{"../escape", "/absolute", ".uap/managed/clients/copilot/../other", ".uap\\managed\\clients\\copilot\\fixture", ".uap/managed/clients/copilot/", ".uap/managed/clients/copilot//extra"} {
		reg.Directory = bad
		if _, _, err := PatchCopilotPluginSettings(nil, reg, false, false); err == nil {
			t.Fatalf("unsafe relative location accepted: %q", bad)
		}
	}
	reg.Directory = ".uap/managed/clients/copilot/fixture"
	reg.Plugin = "foo@bar"
	if _, _, err := InspectCopilotPluginSettings(nil, reg); err == nil {
		t.Fatal("ambiguous enabled-plugin key accepted")
	}
}

func TestCopilotPluginSettingsKeepsIndependentMarketplaces(t *testing.T) {
	a := CopilotPluginRegistration{Marketplace: "agentplugins-aaa", Plugin: "first", Directory: ".uap/managed/clients/copilot/a"}
	b := CopilotPluginRegistration{Marketplace: "agentplugins-bbb", Plugin: "second", Directory: ".uap/managed/clients/copilot/b"}
	first, _, err := PatchCopilotPluginSettings(nil, a, false, false)
	if err != nil {
		t.Fatal(err)
	}
	both, _, err := PatchCopilotPluginSettings(first, b, false, false)
	if err != nil {
		t.Fatal(err)
	}
	after, _, err := PatchCopilotPluginSettings(both, a, true, true)
	if err != nil {
		t.Fatal(err)
	}
	if present, exact, err := InspectCopilotPluginSettings(after, b); err != nil || !present || !exact {
		t.Fatalf("removal damaged sibling: %v %v %v", present, exact, err)
	}
	if strings.Contains(string(after), "first@agentplugins-aaa") {
		t.Fatal("removed package retained enabled key")
	}
}
