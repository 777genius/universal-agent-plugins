package providers_test

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// These production provider observations report only visible user-file rules.
// Project selectors are not native skill policy; session overrides remain unknown.
func TestProjectCodexSkillWarningsRespectPinnedUserSelectorContract(t *testing.T) {
	for _, name := range []string{"disabled-document", "disabled-trimmed-name", "later-name-enable", "later-path-enable", "both-selectors-ignored", "directory-not-document", "project-selector-ignored", "missing-required-enabled", "relative-path-invalidates-section"} {
		t.Run(name, func(t *testing.T) {
			f := newProjectCodexFixture(t)
			request := f.stage(t, domain.CompatibilityHints{})
			document := filepath.Join(f.a, ".agents", "skills", "docs", "SKILL.md")
			pathRule := fmt.Sprintf("[[skills.config]]\npath=%q\nenabled=false\n", document)
			user, project, want := pathRule, "", true
			switch name {
			case "disabled-trimmed-name":
				user = "[[skills.config]]\nname=' docs '\nenabled=false\n"
			case "later-name-enable":
				user += "[[skills.config]]\nname='docs'\nenabled=true\n"
				want = false
			case "later-path-enable":
				user = "[[skills.config]]\nname='docs'\nenabled=false\n" + strings.Replace(pathRule, "enabled=false", "enabled=true", 1)
				want = false
			case "both-selectors-ignored":
				user = strings.Replace(pathRule, "enabled=false", "name='docs'\nenabled=false", 1)
				want = false
			case "directory-not-document":
				user = fmt.Sprintf("[[skills.config]]\npath=%q\nenabled=false\n", filepath.Dir(document))
				want = false
			case "project-selector-ignored":
				user, project, want = "", pathRule, false
			case "missing-required-enabled":
				user += "[[skills.config]]\nname='unrelated'\n"
				want = false
			case "relative-path-invalidates-section":
				user += "[[skills.config]]\npath='relative/SKILL.md'\nname='unrelated'\nenabled=true\n"
				want = false
			}
			projectCodexWrite(t, filepath.Join(f.user, "config.toml"), user)
			if project != "" {
				projectCodexWrite(t, filepath.Join(f.a, ".codex", "config.toml"), project)
			}
			before := projectGeminiInventory(t, f.user)
			outcome, err := f.activator.Activate(t.Context(), request)
			if err != nil || outcome.Activation != domain.ActivationPrepared || outcome.Verification != domain.VerificationInstalled {
				t.Fatalf("configured project = %+v, %v", outcome, err)
			}
			if got := strings.Contains(strings.Join(outcome.UserActions, "\n"), `disable skill "docs"`); got != want {
				t.Fatalf("disabled warning = %t, want %t: %q", got, want, outcome.UserActions)
			}
			if got := projectGeminiInventory(t, f.user); !reflect.DeepEqual(got, before) {
				t.Fatal("selector observation changed user settings")
			}
		})
	}
}

func TestProjectCodexSkillWarningRecognizesCanonicalDocumentAlias(t *testing.T) {
	f := newProjectCodexFixture(t)
	request := f.stage(t, domain.CompatibilityHints{})
	outcome, err := f.activator.Activate(t.Context(), request)
	if err != nil {
		t.Fatal(err)
	}
	document := filepath.Join(f.a, ".agents", "skills", "docs", "SKILL.md")
	alias := filepath.Join(f.a, "inert-skill-document-alias")
	if err := os.Symlink(document, alias); err != nil {
		t.Skipf("document alias unavailable: %v", err)
	}
	projectCodexWrite(t, filepath.Join(f.user, "config.toml"), fmt.Sprintf("[[skills.config]]\npath=%q\nenabled=false\n", alias))
	request.PreviousNativeObjects, request.VerifyOnly = outcome.NativeObjects, true
	outcome, err = f.activator.Activate(t.Context(), request)
	if err != nil || !strings.Contains(strings.Join(outcome.UserActions, "\n"), `disable skill "docs"`) || outcome.Activation != domain.ActivationPrepared {
		t.Fatalf("canonical document selector = %+v, %v", outcome, err)
	}
}
