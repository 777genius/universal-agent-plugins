package agentpluginscli

import (
	"bytes"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli/prompt"
	"github.com/777genius/plugin-kit-ai/cli/internal/terminaltheme"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/skillsource"
)

// A bounded display must keep distant entries reachable and keep a basket when
// the query changes. The stream also verifies that invalid answers do not eat
// the next answer or silently change the selection.
func TestSkillsTerminalSearchPreservesBasketAcrossQueries(t *testing.T) {
	available := make([]skillsource.Candidate, 0, 120)
	for i := range 120 {
		available = append(available, skillsource.Candidate{ID: fmt.Sprint(i), Name: fmt.Sprintf("skill-%03d", i), Path: fmt.Sprintf("category-%03d/skill-%03d", i, i)})
	}
	var output bytes.Buffer
	command := &cobra.Command{}
	command.SetOut(&output)
	command.SetIn(strings.NewReader("done\n99\n1\n/category-119\n1\n/\n1\n1\ndone\n"))
	selected, err := promptFilteredSkills(command, available)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(selected, []skillsource.Candidate{available[0], available[119]}) {
		t.Fatalf("distant selections were lost: %+v", selected)
	}
	for _, expected := range []string{"50 of 120", "2 selected", "[x]", "Select at least one", "Choose unique numbers", "clear removes selections", "done reviews"} {
		if !strings.Contains(output.String(), expected) {
			t.Fatalf("missing guidance %q", expected)
		}
	}
}

func TestSkillsCatalogThemePreservesPlainOutputAndSafeText(t *testing.T) {
	for _, tc := range []struct {
		name, mode string
		plain      bool
		colored    bool
	}{{"rich", "always", false, true}, {"plain", "always", true, false}, {"no-color", "never", false, false}} {
		t.Run(tc.name, func(t *testing.T) {
			var output bytes.Buffer
			format := "human"
			policy := terminaltheme.Policy{Mode: tc.mode, Explicit: true}
			command := &cobra.Command{}
			command.Flags().Bool("plain", tc.plain, "")
			command.SetOut(terminaltheme.Wrap(&output, &policy, &format))
			candidate := skillsource.Candidate{ID: "review", Name: "review\x1b[31m", Path: "engineering/review"}
			if err := renderTerminalSkillMatches(command, []skillsource.Candidate{candidate}, 1, "", map[string]bool{"review": true}); err != nil {
				t.Fatal(err)
			}
			got := output.String()
			if strings.Contains(got, "\x1b[") != tc.colored {
				t.Fatalf("wrong color policy: %q", got)
			}
			if strings.Contains(got, "\x1b[31m") || !strings.Contains(got, "[x]") || !strings.Contains(got, "1 selected") || !strings.Contains(got, "done reviews") {
				t.Fatalf("unsafe text or missing selection guidance: %q", got)
			}
		})
	}
}

func TestSkillsTerminalSearchClearCancelAndEOF(t *testing.T) {
	available := []skillsource.Candidate{{ID: "first", Name: "first"}, {ID: "last", Name: "last"}}
	command := &cobra.Command{}
	command.SetOut(&bytes.Buffer{})
	command.SetIn(strings.NewReader("1\nclear\n2\ndone\n"))
	selected, err := promptFilteredSkills(command, available)
	if err != nil || !reflect.DeepEqual(selected, available[1:]) {
		t.Fatalf("clear did not remove prior selections: %+v %v", selected, err)
	}
	for _, input := range []string{"q\n", "\n", "\x1b\n", ""} {
		command.SetIn(strings.NewReader(input))
		_, err = promptFilteredSkills(command, available)
		if !errors.Is(err, prompt.ErrPromptCanceled) && !errors.Is(err, prompt.ErrPromptInputClosed) {
			t.Fatalf("cancel/EOF did not terminate prompt: %q %v", input, err)
		}
	}
}
