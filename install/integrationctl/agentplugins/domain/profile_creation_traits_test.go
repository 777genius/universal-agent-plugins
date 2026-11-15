package domain

import "testing"

func TestUserProfileCreationPolicyDoesNotAdoptExistingBindings(t *testing.T) {
	for _, definition := range ClientDefinitions() {
		want := definition.ID == ClientCopilot
		if definition.Traits.RecordsUserProfileOnCreate != want {
			t.Errorf("%s creation-only profile policy = %v, want %v", definition.ID, definition.Traits.RecordsUserProfileOnCreate, want)
		}
		if want && definition.Traits.BindsNativeProfileRoot {
			t.Fatal("creation-only policy must not enable existing-profile adoption")
		}
	}
	if ClientTraitsFor("unknown").RecordsUserProfileOnCreate {
		t.Fatal("unknown client received profile-recording policy")
	}
}

// Other clients cannot inherit Copilot's explicit saved project repair policy.
func TestSavedProjectRepairOnAddIsNotGeneric(t *testing.T) {
	for _, definition := range ClientDefinitions() {
		want := definition.ID == ClientCopilot
		if got := ClientTraitsFor(definition.ID).SavedProjectRepairOnAdd; got != want {
			t.Errorf("%s saved project repair opt-in = %t, want %t", definition.ID, got, want)
		}
	}
}
