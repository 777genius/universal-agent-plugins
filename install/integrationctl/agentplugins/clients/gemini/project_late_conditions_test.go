package gemini

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

type geminiLateCondition struct {
	name, scope, body string
	directory, allow  bool
}

func TestGeminiObservedSkillsRecheckLateConditions(t *testing.T) {
	for _, test := range []geminiLateCondition{
		{name: "user-disabled", scope: "user", body: `{"skills":{"enabled":false},"foreign":"retain"}`},
		{name: "workspace-disabled", scope: "workspace", body: `{"skills":{"disabled":["audit"]},"foreign":"retain"}`},
		{name: "user-malformed", scope: "user", body: `{"skills":`},
		{name: "workspace-malformed", scope: "workspace", body: `{"skills":{"enabled":true},"skills":{"enabled":false}}`},
		{name: "workspace-unreadable", scope: "workspace", directory: true},
		{name: "workspace-shadow", scope: "alias", body: "---\nname: audit\ndescription: foreign shadow\n---\nPreserve me.\n"},
		{name: "unrelated-user-setting", scope: "user", body: `{"foreign":"retain"}`, allow: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			f := newGeminiSkillAuthFixture(t)
			unchanged := holdGeminiObservationFiles(t, f)
			receipts := slices.Clone(f.request.Delivery.NativeObjects)
			lateUnchanged := injectGeminiLateCondition(t, &f, test)
			outcome, err := f.complete(t)
			activation, authentication := domain.ActivationPrepared, domain.AuthenticationNotChecked
			if test.allow {
				activation, authentication = domain.ActivationActive, domain.AuthenticationNotRequired
			}
			if err != nil || len(f.probe.commands) != 1 || outcome.Activation != activation || outcome.Authentication != authentication ||
				outcome.Verification != domain.VerificationInstalled || outcome.Policy != domain.PolicyAllowed ||
				outcome.NativeEffect != domain.NativeEffectUnchanged || outcome.ActivationAttested || outcome.AuthenticationAttested {
				t.Fatalf("late conditions were not respected by fresh observation: %+v %v", outcome, err)
			}
			if !test.allow && len(outcome.UserActions) == 0 {
				t.Fatal("refused discovery lost its pending action")
			}
			if !reflect.DeepEqual(receipts, f.request.Delivery.NativeObjects) {
				t.Fatal("late condition changed the owned receipts")
			}
			unchanged()
			lateUnchanged()
		})
	}
}

func injectGeminiLateCondition(t *testing.T, f *geminiSkillAuthFixture, test geminiLateCondition) func() {
	t.Helper()
	path := filepath.Join(f.request.Client.ConfigRoot, "settings.json")
	switch test.scope {
	case "workspace":
		path = filepath.Join(f.request.Plan.NativeRegistryRoot, "settings.json")
	case "alias":
		path = filepath.Join(f.request.Plan.TargetContext.ProjectRoot, ".agents", "skills", "audit", "SKILL.md")
	}
	var written os.FileInfo
	f.probe.after = func() {
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			t.Fatal(err)
		}
		var err error
		if test.directory {
			err = os.Mkdir(path, 0700)
		} else {
			err = os.WriteFile(path, []byte(test.body), 0600)
		}
		if err != nil {
			t.Fatal(err)
		}
		written, err = os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
	}
	return func() {
		t.Helper()
		after, err := os.Stat(path)
		if err != nil || written == nil || !os.SameFile(written, after) || written.Mode() != after.Mode() || !written.ModTime().Equal(after.ModTime()) {
			t.Fatal("observation replaced the caller's late condition", path, err)
		}
		if test.directory {
			entries, readErr := os.ReadDir(path)
			if readErr != nil || len(entries) != 0 {
				t.Fatal("observation changed unreadable settings directory", readErr)
			}
			return
		}
		body, err := os.ReadFile(path)
		if err != nil || string(body) != test.body {
			t.Fatal("observation changed the caller's late condition bytes", path, err)
		}
	}
}

func holdGeminiObservationFiles(t *testing.T, f geminiSkillAuthFixture) func() {
	t.Helper()
	bodies, identities := map[string][]byte{}, map[string]os.FileInfo{}
	for _, path := range []string{f.trust, f.skill} {
		body, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		bodies[path], identities[path] = body, info
	}
	digest, err := shared.DigestSkillDirectory(filepath.Dir(f.skill))
	if err != nil {
		t.Fatal(err)
	}
	return func() {
		t.Helper()
		for path, before := range bodies {
			after, readErr := os.ReadFile(path)
			info, statErr := os.Stat(path)
			old := identities[path]
			if readErr != nil || statErr != nil || !bytes.Equal(before, after) || !os.SameFile(old, info) ||
				old.Mode() != info.Mode() || !old.ModTime().Equal(info.ModTime()) {
				t.Fatal("late condition changed original owned/trust files", path, readErr, statErr)
			}
		}
		after, digestErr := shared.DigestSkillDirectory(filepath.Dir(f.skill))
		if digestErr != nil || after != digest {
			t.Fatal("late condition changed the complete owned Skill", digestErr)
		}
	}
}
