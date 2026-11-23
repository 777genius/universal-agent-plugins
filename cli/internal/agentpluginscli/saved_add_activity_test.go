package agentpluginscli

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/sourcespec"
)

type savedAddActivitySource struct {
	*savedJourneySource
	beforeAcquire func()
}

func (s savedAddActivitySource) AcquireGit(ctx context.Context, source sourcespec.Git, commit, digest string) (domain.PackageSnapshot, error) {
	s.beforeAcquire()
	return s.savedJourneySource.AcquireGit(ctx, source, commit, digest)
}

type savedAddConfirmationReader struct {
	io.Reader
	beforeRead func()
}

func (reader savedAddConfirmationReader) Read(body []byte) (int, error) {
	reader.beforeRead()
	return reader.Reader.Read(body)
}

func TestSavedAddActivityPrecedesAcquisitionAndReview(t *testing.T) {
	repo, commit := savedJourneyRepository(t)
	for _, mode := range []string{"terminal", "nonterminal", "json"} {
		t.Run(mode, func(t *testing.T) {
			j := newSavedJourney(t, repo)
			var out, diagnostics bytes.Buffer
			app := j.cli.app
			app.Terminal = mode != "nonterminal"
			app.Output, app.ErrorOutput = &out, &diagnostics
			acquired, confirmed := false, false
			app.SourceAcquirer = savedAddActivitySource{savedJourneySource: j.source, beforeAcquire: func() {
				acquired = true
				hasActivity := strings.Contains(diagnostics.String(), "Resolving and validating saved dependency")
				if hasActivity != (mode == "terminal") {
					t.Errorf("activity at acquisition in %s: %q", mode, diagnostics.String())
				}
			}}
			app.Input = savedAddConfirmationReader{Reader: strings.NewReader("n\n"), beforeRead: func() {
				confirmed = true
				if !strings.Contains(diagnostics.String(), "Preparing saved dependency review") || !strings.Contains(out.String(), "Apply these changes? [y/N]") {
					t.Error("confirmation preceded preparation activity or review")
				}
			}}
			cmd := &cobra.Command{Use: "add"}
			cmd.SetContext(t.Context())
			cmd.SetIn(app.Input)
			cmd.SetOut(&out)
			cmd.SetErr(&diagnostics)
			opts := &options{format: "human", scope: "user", dryRun: mode != "terminal"}
			if mode == "json" {
				opts.format = "json"
			}
			source := "github:owner/repo@" + commit
			if err := runSavedAdd(cmd, app, opts, source, []domain.ClientID{domain.ClientCodex}, exactGitPattern.FindStringSubmatch(source), false, false); err != nil {
				t.Fatalf("saved add: %v\n%s\n%s", err, out.String(), diagnostics.String())
			}
			if !acquired || confirmed != (mode == "terminal") {
				t.Fatalf("acquired=%v confirmed=%v mode=%s", acquired, confirmed, mode)
			}
			if mode != "terminal" && strings.Contains(diagnostics.String(), "Preparing saved dependency review") {
				t.Fatalf("activity polluted %s: %q", mode, diagnostics.String())
			}
			if strings.Contains(diagnostics.String(), "\x1b") {
				t.Fatalf("non-PTY activity emitted terminal controls: %q", diagnostics.String())
			}
			if mode == "json" && !json.Valid(out.Bytes()) {
				t.Fatalf("invalid JSON: %s", out.String())
			}
			if len(j.state(t).Installations) != 0 {
				t.Fatal("decline/dry run created installation bindings")
			}
			manifest, lock := app.dependencyPaths()
			for _, path := range []string{manifest, lock} {
				if _, err := os.Stat(path); !os.IsNotExist(err) {
					t.Fatalf("decline/dry run published agreement %s: %v", path, err)
				}
			}
		})
	}
}
