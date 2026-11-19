package agentpluginscli

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func TestKimiRawProjectSkillsRepeatHydratesOnlySelectedVersion(t *testing.T) {
	for _, version := range []string{"2.3.2", "2.5.2"} {
		t.Run(version, func(t *testing.T) {
			_, journey, source, _ := managementMutableWebsite(t)
			native := kimiRawJourney(t, &journey, version, true)
			probed, err := journey.cli.app.Detector.Detect(t.Context())
			if err != nil || len(probed) != 1 {
				t.Fatalf("fixture detector: %v %v", probed, err)
			}
			discovered := append([]domain.DetectedClient(nil), probed...)
			discovered[0].Version = ""
			detector := &projectVersionDetector{clients: discovered, probed: probed}
			journey.cli.app.Detector = detector
			argv := []string{"skills", "add", source, "--skill", "pdf", "--target", "kimi", "--no-ui", "--yes", "--scope", "project", "--project-root", native.project, "--format", "json"}
			var held domain.ClientBinding
			for attempt := range 2 {
				body := journey.run(t, argv...)
				var result map[string]any
				if err := json.Unmarshal([]byte(body), &result); err != nil {
					t.Fatal(err)
				}
				items, ok := result["items"].([]any)
				if !ok || len(items) != 1 {
					t.Fatal("missing selected item", body)
				}
				item, ok := items[0].(map[string]any)
				if !ok || item["status"] != "succeeded" || result["readiness"] != "ready" || native.queries != attempt+1 || len(detector.targets) != 2*(attempt+1) || detector.broadCalls != 0 {
					t.Fatalf("versionless Kimi repeat failed exact completion: %s queries=%d versions=%v", body, native.queries, detector.targets)
				}
				binding := savedJourneyBinding(t, journey.state(t), "kimi")
				if attempt == 0 {
					held = binding
				} else if held.ClientBindingID != binding.ClientBindingID || held.TargetLocator != binding.TargetLocator || !reflect.DeepEqual(held.PackageRevision, binding.PackageRevision) {
					t.Fatal("repeat changed binding, target or pinned revision")
				}
			}
			for _, targets := range detector.targets {
				if !reflect.DeepEqual(targets, []domain.ClientID{domain.ClientKimi}) {
					t.Fatal("unrelated version probe", targets)
				}
			}
			if native.registrations != 1 {
				t.Fatal("repeat reregistered exact workspace")
			}
		})
	}
}
