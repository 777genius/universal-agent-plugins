package agentpluginscli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/skillsource"
)

func TestSkillsCLIInvalidFlagsFailBeforeAcquisition(t *testing.T) {
	for _, test := range []struct{ flag, value, code string }{
		{"--path", "../escape", "INVALID_PATH"},
		{"--skill", "review,review", "INVALID_SELECTION"},
	} {
		var output bytes.Buffer
		command := NewRoot(App{Output: &output, ErrorOutput: &bytes.Buffer{}})
		command.SetArgs([]string{"skills", "add", "acme/skills", test.flag, test.value, "--format", "json"})
		if err := command.ExecuteContext(context.Background()); err == nil || !strings.Contains(output.String(), test.code) {
			t.Fatalf("invalid flag reached missing source acquirer: %v %s", err, &output)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	var output bytes.Buffer
	command := NewRoot(App{Output: &output, ErrorOutput: &output})
	command.SetArgs([]string{"skills", "add", "acme/skills", "--no-ui"})
	if err := command.ExecuteContext(ctx); !errors.Is(err, context.Canceled) || output.Len() != 0 {
		t.Fatalf("canceled command started source work: %v %s", err, &output)
	}
}

func TestSkillsCLISelectionErrorsAndCanceledReviewPreserveEnvironment(t *testing.T) {
	journey, repo, _ := skillsFullFlowFixture(t)
	broken := filepath.Join(repo, "docs", "broken")
	if err := os.MkdirAll(broken, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(broken, "SKILL.md"), []byte("---\nname: broken\n---\n"), 0600); err != nil {
		t.Fatal(err)
	}
	savedJourneyGit(t, repo, "add", ".")
	savedJourneyGit(t, repo, "commit", "-m", "test: malformed disposable Skill")
	before := savedJourneyTree(t, journey.cli.app.DependenciesRoot)
	for _, test := range []struct{ flag, value, code, detail string }{
		{"--skill", "absent", "SKILL_NOT_FOUND", "Names are exact"},
		{"--skill", "review", "SKILL_AMBIGUOUS", "docs/review"},
		{"--path", "docs/broken", "SKILL_INVALID", "name and description"},
	} {
		out, _, err := journey.cli.execute(false, "skills", "add", "acme/skills", test.flag, test.value, "--target", "codex", "--no-ui", "--yes", "--format", "json")
		if err == nil || !strings.Contains(out, test.code) || !strings.Contains(out, test.detail) {
			t.Fatalf("wanted actionable %s refusal: %v %s", test.code, err, out)
		}
	}
	out, stderr, err := journey.cli.executeInput(true, "/engineering\n99\n1\n/docs/review\n1\ndone\nn\n", "skills", "add", "acme/skills", "--target", "codex", "--no-ui")
	if err != nil || !strings.Contains(out, "2 selected") || !strings.Contains(out, "Installation canceled") || !strings.Contains(stderr, "Scanning Skills catalog") {
		t.Fatalf("search/review/cancel flow failed: %v %s %s", err, out, stderr)
	}
	if !reflect.DeepEqual(before, savedJourneyTree(t, journey.cli.app.DependenciesRoot)) || journey.runner.calls != 0 {
		t.Fatal("refused or canceled review changed the environment")
	}
}

// Unknown metadata must remain distinguishable from a genuine empty resource.
func TestSkillsResourceWireDistinguishesUnknownAndEmpty(t *testing.T) {
	zero := int64(0)
	resources := []skillsource.Resource{{Path: "unknown.bin"}, {Path: "empty.txt", Size: &zero}}
	body, err := json.Marshal([]any{wireSkillResource(resources[0]), wireSkillResource(resources[1])})
	if err != nil {
		t.Fatal(err)
	}
	var values []map[string]any
	if err := json.Unmarshal(body, &values); err != nil {
		t.Fatal(err)
	}
	if _, present := values[0]["size"]; present || values[1]["size"] != float64(0) {
		t.Fatalf("unknown and empty sizes collapsed: %s", body)
	}
}
