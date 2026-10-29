package agentpluginscli

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/environment"
)

type targetNoticeWriter struct{ err error }

func (w targetNoticeWriter) Write([]byte) (int, error) { return 0, w.err }

func TestSavedTargetNoticeOutput(t *testing.T) {
	lock := environment.Lock{Packages: []environment.LockedPackage{{Alias: "demo", Targets: []string{"codex", "claude"}}}}
	for _, format := range []string{"human", "json"} {
		t.Run(format, func(t *testing.T) {
			var out bytes.Buffer
			cmd := &cobra.Command{}
			cmd.SetOut(&out)
			opts := &options{format: format, environmentOutput: map[string]any{}}
			if err := renderSavedTargetNotice(cmd, opts, lock, []string{"demo"}, []string{"codex"}); err != nil {
				t.Fatal(err)
			}
			notice, _ := opts.environmentOutput["target_notice"].(string)
			if !strings.Contains(notice, "Saved targets claude are preserved") {
				t.Fatal(notice)
			}
			if format == "human" && out.String() != notice+"\n" {
				t.Fatal(out.String())
			}
			if format == "json" && out.Len() != 0 {
				t.Fatal("human text in JSON output")
			}
		})
	}
	failure := errors.New("closed output")
	cmd := &cobra.Command{}
	cmd.SetOut(targetNoticeWriter{err: failure})
	if err := renderSavedTargetNotice(cmd, &options{format: "human"}, lock, []string{"demo"}, []string{"codex"}); !errors.Is(err, failure) {
		t.Fatal(err)
	}
}
