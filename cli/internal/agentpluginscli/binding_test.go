package agentpluginscli

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli/prompt"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/usecase"
	"github.com/spf13/cobra"
)

func bindingReviewFixture(t *testing.T, mode usecase.BindingChangeMode) (cliFixture, string) {
	t.Helper()
	fixture := newCLIFixture(t, nil)
	installation := domain.Installation{
		InstallationID: "00000000-0000-4000-8000-000000000009", DeclaredName: "demo", NeedsRebind: true,
		Source:  domain.SourceBinding{SourceBindingID: "src_old", RequestedSource: "old", CanonicalSource: "https://example.test/old", TreeDigest: "sha256:old"},
		Package: domain.PackageBinding{LoaderKind: domain.LoaderKindAgentPlugins, FormatID: domain.FormatIDAgentPluginsV1, SchemaURI: domain.PluginSchemaV1, DeclaredName: "demo", Version: "0.9.0", ManifestDigest: "sha256:old"},
		Clients: map[string]domain.ClientBinding{},
	}
	if mode == usecase.BindingChangeMigrateFormat {
		installation.Package.LoaderKind = domain.LoaderKindLegacy
		installation.Package.FormatID = domain.FormatIDLegacyV1
		installation.Package.SchemaURI = "plugin.yaml/v1"
	}
	if err := fixture.store.Save(domain.StateFileV2{SchemaVersion: domain.StateSchemaVersion, Installations: []domain.Installation{installation}}); err != nil {
		t.Fatal(err)
	}
	return fixture, writeCLIPlugin(t)
}

type bindingConsentReader struct {
	reads  int
	before func()
	input  *strings.Reader
}

func (r *bindingConsentReader) Read(p []byte) (int, error) {
	r.reads++
	if r.before != nil {
		r.before()
	}
	return r.input.Read(p)
}

type bindingFailWriter struct {
	err       error
	remaining int
}

func (w *bindingFailWriter) Write(p []byte) (int, error) {
	if w.remaining > 0 {
		w.remaining--
		return len(p), nil
	}
	if w.err != nil {
		return 0, w.err
	}
	return len(p) - 1, nil
}

func runBindingReview(t *testing.T, fixture cliFixture, source string, mode usecase.BindingChangeMode, opts options, terminal bool, input io.Reader, out, stderr io.Writer) error {
	t.Helper()
	cmd := &cobra.Command{}
	cmd.SetIn(input)
	cmd.SetOut(out)
	cmd.SetErr(stderr)
	cmd.SetContext(context.Background())
	app := fixture.app
	app.Terminal = terminal
	return runBindingChange(context.Background(), cmd, app, &opts, mode, "demo", source)
}

func TestBindingReviewConsentContracts(t *testing.T) {
	for _, mode := range []usecase.BindingChangeMode{usecase.BindingChangeRebind, usecase.BindingChangeMigrateFormat} {
		for _, tc := range []struct {
			name, input, format            string
			terminal, dryRun, mutate, read bool
		}{
			{"approve", "y\n", "human", true, false, true, true},
			{"decline", "n\n", "human", true, false, false, true},
			{"json", "", "json", true, false, true, false},
			{"nonterminal", "", "human", false, false, true, false},
			{"preview", "", "human", true, true, false, false},
		} {
			t.Run(bindingCommandName(mode)+"/"+tc.name, func(t *testing.T) {
				fixture, source := bindingReviewFixture(t, mode)
				before, _ := fixture.store.Load()
				var out, stderr bytes.Buffer
				input := &bindingConsentReader{input: strings.NewReader(tc.input)}
				input.before = func() {
					if !strings.Contains(out.String(), "PLUGIN_DATA: not transferred") {
						t.Error("consent read before complete plan")
					}
				}
				err := runBindingReview(t, fixture, source, mode, options{format: tc.format, dryRun: tc.dryRun}, tc.terminal, input, &out, &stderr)
				if err != nil {
					t.Fatal(err)
				}
				after, err := fixture.store.Load()
				if err != nil {
					t.Fatal(err)
				}
				if changed := !reflect.DeepEqual(before, after); changed != tc.mutate {
					t.Fatalf("mutated=%v want %v", changed, tc.mutate)
				}
				if (input.reads > 0) != tc.read {
					t.Fatalf("consent reads=%d", input.reads)
				}
				if tc.format == "json" && (strings.Contains(out.String(), "Plugin:") || strings.Contains(out.String(), "[y/N]")) {
					t.Fatal("human output in JSON")
				}
			})
		}
	}
}

func TestBindingReviewWriterFailurePreventsConsentAndMutation(t *testing.T) {
	failure := errors.New("plan writer failed")
	for _, mode := range []usecase.BindingChangeMode{usecase.BindingChangeRebind, usecase.BindingChangeMigrateFormat} {
		for _, short := range []bool{false, true} {
			for _, terminal := range []bool{false, true} {
				fixture, source := bindingReviewFixture(t, mode)
				before, _ := fixture.store.Load()
				want := failure
				writer := &bindingFailWriter{err: failure, remaining: 3}
				if short {
					writer.err = nil
					want = io.ErrShortWrite
				}
				input := &bindingConsentReader{input: strings.NewReader("y\n")}
				err := runBindingReview(t, fixture, source, mode, options{format: "human"}, terminal, input, writer, io.Discard)
				if !errors.Is(err, want) {
					t.Fatalf("%s error=%v want %v", bindingCommandName(mode), err, want)
				}
				after, _ := fixture.store.Load()
				if input.reads != 0 || !reflect.DeepEqual(before, after) {
					t.Fatal("failed plan read consent or mutated state")
				}
			}
		}
	}
}

func TestBindingReviewMissingVisiblePlan(t *testing.T) {
	for _, mode := range []usecase.BindingChangeMode{usecase.BindingChangeRebind, usecase.BindingChangeMigrateFormat} {
		fixture, source := bindingReviewFixture(t, mode)
		before, _ := fixture.store.Load()
		out, err := os.CreateTemp(t.TempDir(), "redirected")
		if err != nil {
			t.Fatal(err)
		}
		defer out.Close()
		input := &bindingConsentReader{input: strings.NewReader("y\n")}
		err = runBindingReview(t, fixture, source, mode, options{format: "human"}, true, input, out, out)
		if !errors.Is(err, prompt.ErrPromptUnavailable) {
			t.Fatalf("error=%v", err)
		}
		after, _ := fixture.store.Load()
		info, _ := out.Stat()
		if input.reads != 0 || !reflect.DeepEqual(before, after) || info.Size() != 0 {
			t.Fatal("unavailable plan read consent, mutated, or wrote redirected plan")
		}
	}
}

func TestBindingPlanRenderingPropagatesErrors(t *testing.T) {
	for _, dryRun := range []bool{false, true} {
		err := renderBindingChange(&bindingFailWriter{}, "human", "rebind", usecase.BindingChangeResult{}, dryRun)
		if !errors.Is(err, io.ErrShortWrite) {
			t.Fatalf("error=%v", err)
		}
	}
}
