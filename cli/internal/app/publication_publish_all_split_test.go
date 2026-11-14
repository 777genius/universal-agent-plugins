package app

import "testing"

func TestValidatePublishAllOptionsDefaultsRootForDryRun(t *testing.T) {
	t.Parallel()

	root, err := validatePublishAllOptions(PluginPublishOptions{DryRun: true})
	if err != nil {
		t.Fatal(err)
	}
	if root != "." {
		t.Fatalf("root = %q", root)
	}
}

func TestBuildPublishPlanResultTracksWarningsAndChannels(t *testing.T) {
	t.Parallel()

	result := buildPublishPlanResult(PluginPublishOptions{Dest: "/tmp/out"}, nil, publishAllPlan{
		results:  []PluginPublishResult{{Channel: "gemini-gallery"}},
		warnings: []string{"warn"},
		next:     []string{"next"},
		ready:    false,
	})
	if result.Status != "needs_attention" || result.WarningCount != 1 || result.ChannelCount != 1 {
		t.Fatalf("result = %+v", result)
	}
}
