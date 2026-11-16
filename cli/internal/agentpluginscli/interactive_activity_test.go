package agentpluginscli

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

type activityObservingDetector struct{ observe func() }

func (detector activityObservingDetector) Detect(context.Context) ([]domain.DetectedClient, error) {
	detector.observe()
	return nil, errors.New("fixture detection stopped")
}

func TestInteractiveAddShowsActivityBeforeDetection(t *testing.T) {
	var output bytes.Buffer
	command := &cobra.Command{}
	command.SetErr(&output)
	app := App{Terminal: true, Detector: activityObservingDetector{observe: func() {
		if !strings.Contains(output.String(), "Finding installed agents") {
			t.Fatal("detection began before the user received a status")
		}
	}}}
	_, _, _, err := promptCompatibleDetectedTargets(t.Context(), command, app, &options{format: "human", plain: true}, "fixture")
	if err == nil || !strings.Contains(err.Error(), "fixture detection stopped") || strings.Contains(output.String(), "\x1b") {
		t.Fatalf("detection failure/plain status changed: %v %q", err, output.String())
	}
}
