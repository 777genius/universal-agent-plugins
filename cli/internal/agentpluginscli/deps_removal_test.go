package agentpluginscli

import (
	"bytes"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/environment"
)

func TestSavedRemovalGraphIntentAndIdentity(t *testing.T) {
	for _, tc := range []struct {
		name      string
		operation string
		wantError bool
	}{
		{name: "metadata changes preserve unselected identity", operation: "metadata"},
		{name: "immutable changes fail", operation: "identity", wantError: true},
		{name: "saved selection includes old and new closure", operation: "selection"},
		{name: "transitive registry add becomes direct without acquisition", operation: "promotion"},
		{name: "remove direct assignment keeps other target", operation: "target"},
		{name: "pure transitive removal reports consumers", operation: "retention"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			pin := environment.LockedPackage{Alias: "c", Version: "1.2.0", SourceKey: "directory:acme/c", TreeDigest: "sha256:" + strings.Repeat("a", 64), Targets: []string{"codex"},
				Source:     environment.LockedSource{Registry: &environment.LockedRegistrySource{Distribution: "acme/c", DistributionID: "acme/c", ReleaseSequence: 3, Requested: environment.Selector{Kind: environment.SelectorRelease, Value: "3"}, Commit: strings.Repeat("1", 40)}},
				RequiredBy: []environment.RequiredBy{{Alias: "a", Selector: environment.Selector{Kind: environment.SelectorRange, Value: "^1.2"}}}}
			old := environment.Lock{Packages: []environment.LockedPackage{pin}}
			switch tc.operation {
			case "metadata", "identity":
				changed := pin
				source := *pin.Source.Registry
				changed.Source.Registry = &source
				changed.Targets = []string{"claude"}
				changed.RequiredBy = nil
				changed.Source.Registry.Requested = environment.Selector{Kind: environment.SelectorRange, Value: "^1.2"}
				if tc.operation == "identity" {
					changed.Source.Registry.Commit = strings.Repeat("2", 40)
				}
				err := verifyUnselectedPins(old, environment.Lock{Packages: []environment.LockedPackage{changed}}, []string{"a"})
				if (err != nil) != tc.wantError {
					t.Fatalf("err=%v wantError=%v", err, tc.wantError)
				}
			case "selection":
				next := environment.Lock{Packages: []environment.LockedPackage{{Alias: "d", RequiredBy: []environment.RequiredBy{{Alias: "a"}}}}}
				if got := savedClosureAliases(old, next, []string{"a"}); !slices.Equal(got, []string{"a", "c", "d"}) {
					t.Fatalf("selection=%v", got)
				}
			case "promotion":
				req, got, err := resolveSavedAdd(&cobra.Command{}, App{}, &options{}, "acme/c", []domain.ClientID{domain.ClientCodex}, nil, environmentSnapshot{Lock: old})
				if err != nil {
					t.Fatal(err)
				}
				if req.Alias != "c" || req.Registry == nil || req.Registry.Distribution != "acme/c" || req.Registry.Selector != (environment.Selector{Kind: environment.SelectorRelease, Value: "3"}) {
					t.Fatalf("root=%+v", req)
				}
				if !reflect.DeepEqual(pin, got) {
					t.Fatal("promoting transitive dependency changed pin")
				}
			case "target":
				req := environment.Requirements{SchemaVersion: 1, Dependencies: map[string]environment.Requirement{"c": {Alias: "c", Registry: &environment.RegistrySource{Distribution: "acme/c", Selector: environment.Selector{Kind: environment.SelectorRelease, Value: "3"}}}}, Targets: map[string][]string{"codex": {"c"}, "claude": {"c"}}}
				next := removeSavedTargets(req, "c", []string{"codex"})
				if _, exists := next.Dependencies["c"]; !exists || !slices.Equal(next.Targets["claude"], []string{"c"}) || len(next.Targets["codex"]) != 0 {
					t.Fatalf("intent=%+v", next)
				}
				final := removeSavedTargets(next, "c", []string{"claude"})
				if len(final.Dependencies) != 0 {
					t.Fatal("last direct target retained root")
				}
			case "retention":
				cmd := &cobra.Command{}
				output := &bytes.Buffer{}
				cmd.SetOut(output)
				if err := renderSavedRetention(cmd, pin); err != nil {
					t.Fatal(err)
				}
				if !strings.Contains(output.String(), "c retained: required by a") {
					t.Fatalf("reason=%q", output.String())
				}
			}
		})
	}
}
