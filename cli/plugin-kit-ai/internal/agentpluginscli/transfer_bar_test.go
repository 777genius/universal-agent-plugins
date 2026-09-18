package agentpluginscli

import (
	"bytes"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/777genius/plugin-kit-ai/cli/internal/terminaltheme"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/sourceacquisition"
)

func TestRenderTransferLineShowsPercentAndPhase(t *testing.T) {
	t.Parallel()
	bar := newTransferBar(&bytes.Buffer{}, "Downloading example/plugin", false)
	empty := renderTransferLine(bar.theme, bar.model, "Downloading example/plugin", "", 0)
	mid := renderTransferLine(bar.theme, bar.model, "Downloading example/plugin", "receiving objects", 0.4)
	full := renderTransferLine(bar.theme, bar.model, "Downloading example/plugin", "done", 1)
	if !strings.Contains(empty, "Downloading example/plugin") || !strings.Contains(empty, "  0%") {
		t.Fatalf("empty bar = %q", empty)
	}
	if !strings.Contains(mid, " 40%") || !strings.Contains(mid, "receiving objects") {
		t.Fatalf("mid bar = %q", mid)
	}
	if strings.Count(mid, "█") <= strings.Count(empty, "█") {
		t.Fatalf("bar did not fill: empty=%q mid=%q", empty, mid)
	}
	if !strings.Contains(full, "100%") || strings.Contains(full, "receiving objects") {
		t.Fatalf("full bar = %q", full)
	}
}

func TestTransferBarRewritesInPlace(t *testing.T) {
	t.Parallel()
	var stderr bytes.Buffer
	bar := newTransferBar(&stderr, "Downloading example/plugin", true)
	bar.Report(sourceacquisition.Transfer{Phase: "receiving objects", Fraction: 0.25})
	bar.Report(sourceacquisition.Transfer{Phase: "receiving objects", Fraction: 0.75})
	bar.Finish(nil)
	got := stderr.String()
	if !strings.Contains(got, "\033[2K") || !strings.Contains(got, "75%") || !strings.Contains(got, "100%") {
		t.Fatalf("live bar = %q", got)
	}
}

func TestStartTransferBarStaysOffOutsideLiveTTY(t *testing.T) {
	t.Parallel()
	format := "human"
	app := App{Terminal: true, ErrorOutput: &bytes.Buffer{}, progressFormat: &format}
	if bar := startTransferBar(app, "Downloading example/plugin"); bar != nil {
		t.Fatal("buffer stderr must not start a live transfer bar")
	}
	jsonFormat := "json"
	app.progressFormat = &jsonFormat
	if bar := startTransferBar(app, "Downloading example/plugin"); bar != nil {
		t.Fatal("json output must not start a live transfer bar")
	}
}

func TestLiveTransferBarPreview(t *testing.T) {
	if os.Getenv("AGENTPLUGINS_TRANSFER_DEMO") != "1" {
		t.Skip("set AGENTPLUGINS_TRANSFER_DEMO=1 and pass -v; go test hides passing output without -v")
	}
	format := "human"
	policy := terminaltheme.Policy{Mode: "always", Explicit: true}
	writer := terminaltheme.Wrap(os.Stderr, &policy, &format)
	bar := newTransferBar(writer, "Downloading example/plugin", true)
	defer bar.Finish(nil)
	for _, frame := range []sourceacquisition.Transfer{
		{Phase: "counting objects", Fraction: 0.1},
		{Phase: "receiving objects", Fraction: 0.35},
		{Phase: "receiving objects", Fraction: 0.6},
		{Phase: "receiving objects", Fraction: 0.85},
		{Phase: "resolving deltas", Fraction: 1},
	} {
		bar.Report(frame)
		time.Sleep(350 * time.Millisecond)
	}
	time.Sleep(800 * time.Millisecond)
}
