package agentpluginscli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/sourceacquisition"
)

func TestSkillsListIsDeterministicAndReadOnly(t *testing.T) {
	journey, _, _ := skillsFullFlowFixture(t)
	before := savedJourneyTree(t, journey.cli.app.DependenciesRoot)
	var first string
	for i := range 2 {
		out, _, err := journey.cli.execute(false, "skills", "list", "acme/skills", "--format", "json")
		if err != nil {
			t.Fatalf("source listing: %v", err)
		}
		var response struct {
			SchemaVersion int              `json:"schema_version"`
			Command       string           `json:"command"`
			Result        string           `json:"result"`
			Data          skillsListReport `json:"data"`
		}
		if err := json.Unmarshal([]byte(out), &response); err != nil {
			t.Fatal(err)
		}
		if response.SchemaVersion != 1 || response.Command != "skills list" || response.Result != "success" {
			t.Fatalf("invalid listing envelope: %s", out)
		}
		if !response.Data.Complete || len(response.Data.Candidates) != 2 {
			t.Fatalf("expected complete nested catalog, got: %s", out)
		}
		paths := []string{response.Data.Candidates[0].Path, response.Data.Candidates[1].Path}
		if !reflect.DeepEqual(paths, []string{"docs/review", "engineering/review"}) {
			t.Fatalf("lost duplicate Skill names or deterministic paths: %v", paths)
		}
		for _, candidate := range response.Data.Candidates {
			if candidate.Name != "review" || candidate.Status != "available" {
				t.Fatalf("unexpected Skill: %+v", candidate)
			}
		}
		if i == 0 {
			first = out
		} else if first != out {
			t.Fatalf("machine listing contains a transient session identifier:\n%s\n%s", first, out)
		}
	}
	if !reflect.DeepEqual(before, savedJourneyTree(t, journey.cli.app.DependenciesRoot)) {
		t.Fatal("read-only listing changed saved environment")
	}
	if journey.runner.calls != 0 {
		t.Fatal("read-only listing started a native client")
	}
}

func TestSkillsListExactFolderAndInvalidSelectors(t *testing.T) {
	journey, _, _ := skillsFullFlowFixture(t)
	out, _, err := journey.cli.execute(false, "skills", "list", "acme/skills",
		"--path", "engineering/review", "--format", "json")
	if err != nil || !strings.Contains(out, `"path":"engineering/review"`) {
		t.Fatalf("exact listing: %v %s", err, out)
	}
	var response struct {
		Data skillsListReport `json:"data"`
	}
	if err := json.Unmarshal([]byte(out), &response); err != nil || len(response.Data.Candidates) != 1 {
		t.Fatalf("exact folder listing: %v %s", err, out)
	}
	for _, args := range [][]string{
		{"skills", "list", "acme/skills", "--path", "../escape", "--format", "json"},
		{"skills", "list", "acme/skills", "--target", "codex", "--format", "json"},
	} {
		if _, _, err := journey.cli.execute(false, args...); err == nil {
			t.Fatalf("unsafe or mutation-only selection unexpectedly accepted: %v", args)
		}
	}
}

func TestAgentYesIsExplicitWithoutChangingLegacyConfirmation(t *testing.T) {
	if !mutationConfirmed(App{Terminal: true}, &options{format: "human", yes: true}) {
		t.Fatal("--yes must explicitly confirm an already reviewed plan")
	}
	if mutationConfirmed(App{Terminal: true}, &options{format: "human"}) {
		t.Fatal("ordinary interactive invocation must still ask")
	}
	if !mutationConfirmed(App{Terminal: false}, &options{format: "json"}) {
		t.Fatal("legacy noninteractive add compatibility changed")
	}
}

type httpsSkillsListStub struct {
	SourceAcquirer
	catalog sourceacquisition.HTTPSkillCatalog
	err     error
}

func (stub httpsSkillsListStub) ListHTTPSkills(context.Context, string) (sourceacquisition.HTTPSkillCatalog, error) {
	return stub.catalog, stub.err
}

func TestSkillsListHTTPSPublisherReadOnlyAndSorted(t *testing.T) {
	fixture := newCLIFixture(t, nil)
	fixture.app.SourceAcquirer = httpsSkillsListStub{
		SourceAcquirer: fixture.app.SourceAcquirer,
		catalog: sourceacquisition.HTTPSkillCatalog{
			URL: "https://publisher.example", IndexSHA256: strings.Repeat("a", 64),
			Candidates: []sourceacquisition.HTTPSkillCandidate{
				{Name: "zeta", Available: false, Reason: "Incomplete publisher file list"},
				{Name: "alpha", Description: "Example", Available: true},
			},
		},
	}
	output, _, err := fixture.execute(false, "skills", "list", "https://publisher.example", "--format", "json")
	if err != nil {
		t.Fatal(err)
	}
	var document struct {
		SchemaVersion int              "json:\"schema_version\""
		Result        string           "json:\"result\""
		Data          skillsListReport "json:\"data\""
	}
	if err := json.Unmarshal([]byte(output), &document); err != nil {
		t.Fatal(err)
	}
	if document.SchemaVersion != 1 || document.Result != "success" || !document.Data.Complete ||
		len(document.Data.Candidates) != 2 || document.Data.Candidates[0].Path != "alpha" ||
		document.Data.Candidates[1].Status != "unavailable" ||
		document.Data.Candidates[1].Reason != "Incomplete publisher file list" {
		t.Fatalf("HTTPS catalog was not rendered deterministically: %s", output)
	}
	human, _, err := fixture.execute(false, "skills", "list", "https://publisher.example")
	if err != nil || !strings.Contains(human, "alpha: alpha (available)") ||
		!strings.Contains(human, "zeta: zeta (unavailable)") {
		t.Fatalf("HTTPS human listing: %v %q", err, human)
	}
	state, err := fixture.store.Load()
	if err != nil || len(state.Installations) != 0 {
		t.Fatalf("HTTPS listing mutated installation state: %v %+v", err, state.Installations)
	}
}

func TestSkillsListHTTPSPublisherFailureEnvelopeAndNoLeaks(t *testing.T) {
	tests := []struct {
		name, source, code string
		failure            error
		retryable          bool
	}{
		{name: "transport", source: "https://publisher.example", code: "SOURCE_UNAVAILABLE",
			failure: errors.New("private-token-canary: upstream handshake failed")},
		{name: "timeout", source: "https://publisher.example", code: "TIMEOUT",
			failure: fmt.Errorf("private-token-canary: %w", context.DeadlineExceeded), retryable: true},
		{name: "cancellation", source: "https://publisher.example", code: "CANCELED",
			failure: context.Canceled},
		{name: "invalid source", source: "https://user:private-token-canary@publisher.example", code: "INVALID_SOURCE"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fixture := newCLIFixture(t, nil)
			fixture.app.SourceAcquirer = httpsSkillsListStub{
				SourceAcquirer: fixture.app.SourceAcquirer, err: tt.failure,
			}
			_, _, err := fixture.execute(false, "skills", "list", tt.source, "--format", "json")
			var usage CLIUsageError
			if !errors.As(err, &usage) || usage.Code != tt.code || !ShouldWriteAgentFailure(err) {
				t.Fatalf("expected %s typed failure, got %v", tt.code, err)
			}
			if strings.Contains(err.Error(), "private-token-canary") {
				t.Fatalf("untrusted source secret leaked in error: %v", err)
			}
			var output bytes.Buffer
			if err := WriteAgentFailure(&output, usage); err != nil {
				t.Fatal(err)
			}
			var envelope struct {
				SchemaVersion int                     "json:\"schema_version\""
				Result        string                  "json:\"result\""
				Data          map[string]AgentFailure "json:\"data\""
			}
			if err := json.Unmarshal(output.Bytes(), &envelope); err != nil {
				t.Fatal(err)
			}
			failure := envelope.Data["error"]
			if envelope.SchemaVersion != 1 || envelope.Result != "failure" ||
				failure.Code != tt.code || failure.Retryable != tt.retryable ||
				strings.Contains(output.String(), "private-token-canary") {
				t.Fatalf("unsafe HTTPS machine failure: %s", output.String())
			}
		})
	}
}

func TestSkillsListHTTPSPublisherRejectsInvalidSelectorsAndUnavailableAdapter(t *testing.T) {
	for _, flags := range [][]string{{"--ref", "main"}, {"--path", "docs"}} {
		fixture := newCLIFixture(t, nil)
		args := append([]string{"skills", "list", "https://publisher.example", "--format", "json"}, flags...)
		_, _, err := fixture.execute(false, args...)
		var usage CLIUsageError
		if !errors.As(err, &usage) || usage.Code != "INVALID_SELECTION" {
			t.Fatalf("HTTPS listing accepted invalid selectors %v: %v", flags, err)
		}
	}
	fixture := newCLIFixture(t, nil)
	fixture.app.SourceAcquirer = &countingSourceAcquirer{delegate: fixture.app.SourceAcquirer}
	_, _, err := fixture.execute(false, "skills", "list", "https://publisher.example", "--format", "json")
	var usage CLIUsageError
	if !errors.As(err, &usage) || usage.Code != "SOURCE_UNAVAILABLE" {
		t.Fatalf("missing HTTPS source adapter returned %v", err)
	}
}
