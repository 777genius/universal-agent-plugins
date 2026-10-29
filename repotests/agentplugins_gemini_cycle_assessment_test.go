//go:build linux

package pluginkitairepo_test

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

type geminiCycleAssessment struct {
	Slot, Key string
	Body      []byte
}

func geminiCycleSourceOperation(label string) bool {
	switch label {
	case "add-user", "add-a", "add-b", "repeat-add-a", "unchanged-update-a", "intact-repair-a", "update-a", "missing-repair-a":
		return true
	default:
		return false
	}
}
func geminiCycleAssessmentFor(r *geminiCycleRecipe) (geminiCycleAssessment, error) {
	if r == nil || !map[string]bool{"user/U-one": true, "a/A-one": true, "b/B-one": true, "a/A-two": true}[r.Scope+"/"+r.Revision] || !geminiCycleDigest(r.Digest) || len(r.Authored["plugin.json"]) == 0 {
		return geminiCycleAssessment{}, fmt.Errorf("Gemini independently held assessment recipe")
	}
	// Pinned fixture requirement and the measured harmless three-file report.
	// The real public CLI still executes its scanner/evaluator and policy gate.
	assessment := domain.SecurityAssessment{
		SchemaVersion: 1,
		Subject:       domain.SecuritySubject{TreeDigest: r.Digest, ManifestDigest: "sha256:" + geminiCycleSHA(r.Authored["plugin.json"])},
		Scanner:       domain.SecurityScanner{ID: "lintai", Version: "0.1.3"},
		Policy:        domain.SecurityPolicy{ID: "agent-plugin-install", Version: 2, Digest: "sha256:9cf869e299d847d7078aeca01f5a182fcdb0144bbf513c83290459991c79e037"},
		Outcome:       domain.SecurityNoBlockingFindings, ScannedFiles: 3,
		ReportDigest: "sha256:90f5acb6ff3ac64f669be4f4ba6deea7ae5aa3a9954662441c0e5498b8d56598",
	}
	key := geminiCycleSHA([]byte(strings.Join([]string{assessment.Subject.TreeDigest, assessment.Subject.ManifestDigest, assessment.Scanner.ID, assessment.Scanner.Version, assessment.Policy.ID, "2", assessment.Policy.Digest}, "\x00")))
	body, err := json.Marshal(assessment)
	if err != nil {
		return geminiCycleAssessment{}, err
	}
	return geminiCycleAssessment{Slot: r.Scope + "/" + r.Revision, Key: key, Body: body}, nil
}
func geminiCycleDigest(value string) bool {
	if len(value) != 71 || !strings.HasPrefix(value, "sha256:") {
		return false
	}
	for _, c := range value[7:] {
		if c < '0' || c > '9' && c < 'a' || c > 'f' {
			return false
		}
	}
	return true
}
