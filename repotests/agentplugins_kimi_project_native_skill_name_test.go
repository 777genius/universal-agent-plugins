//go:build linux

package pluginkitairepo_test

import (
	"fmt"
	"path/filepath"
	"reflect"
	"testing"
)

// The pinned native catalog qualifies plugin skills, while direct project
// skills retain their authored name. This constructs an expectation only;
// package/receipt/file names and incoming native values are never rewritten.
func kimiProjectExpectedSkill(scope string, p *kimiProjectNativePackage) (map[string]any, error) {
	switch scope {
	case "user", "a", "b", "c":
	default:
		return nil, fmt.Errorf("unregistered Kimi skill expectation scope")
	}
	if p == nil || p.Proof == nil || p.Name != "uap-kimi-native-"+scope || p.Proof.scope != scope || p.Proof.name != p.Name || !filepath.IsAbs(p.Skill) || filepath.Clean(p.Skill) != p.Skill {
		return nil, fmt.Errorf("Kimi skill expectation differs from selected recipe")
	}
	file, exists := p.Proof.authored["skills/"+p.Name+"/SKILL.md"]
	if !exists || len(file.Body) == 0 {
		return nil, fmt.Errorf("Kimi skill expectation requires the frozen authored leaf")
	}
	name, source := p.Name, "project"
	if scope == "user" {
		name, source = p.Name+":"+p.Name, "extra"
	}
	return map[string]any{"name": name, "description": "Kimi installed " + p.Revision, "path": p.Skill, "source": source, "sha256": kimiProjectProofSHA(file.Body)}, nil
}

func TestKimiProjectExpectedSkillBindsActualStagerWithoutRenamingAuthority(t *testing.T) {
	for _, scope := range []string{"user", "a", "b", "c"} {
		t.Run(scope, func(t *testing.T) {
			f := newKimiProjectProofContract(t, scope)
			validated, err := f.proof.validate(f.ctx, f.installation)
			if err != nil {
				t.Fatal(err)
			}
			p := f.fixture.packages[scope]
			p.Skill = validated.skill
			before, err := kimiProjectProofCapture(f.ctx, validated.authority.active)
			if err != nil {
				t.Fatal(err)
			}
			row, err := kimiProjectExpectedSkill(scope, p)
			wantName, wantSource := "uap-kimi-native-"+scope, "project"
			if scope == "user" {
				wantName, wantSource = "uap-kimi-native-user:uap-kimi-native-user", "extra"
			}
			body := f.proof.authored["skills/uap-kimi-native-"+scope+"/SKILL.md"].Body
			want := map[string]any{"name": wantName, "description": "Kimi installed " + scope + "-one", "path": validated.skill, "source": wantSource, "sha256": kimiProjectProofSHA(body)}
			if err != nil || !reflect.DeepEqual(row, want) || p.Name != "uap-kimi-native-"+scope || f.installation.DeclaredName != p.Name {
				t.Fatal("native skill name did not bind the selected unchanged recipe", err)
			}
			after, captureErr := kimiProjectProofCapture(f.ctx, validated.authority.active)
			again, proofErr := f.proof.validate(f.ctx, f.installation)
			if captureErr != nil || proofErr != nil || !reflect.DeepEqual(before, after) || !reflect.DeepEqual(validated, again) {
				t.Fatal("native name expectation changed installed payload or receipt authority", captureErr, proofErr)
			}
		})
	}
}

func TestKimiProjectExpectedSkillRejectsUnregisteredOrMismatchedRecipe(t *testing.T) {
	f := newKimiProjectProofContract(t, "user")
	validated, err := f.proof.validate(f.ctx, f.installation)
	if err != nil {
		t.Fatal(err)
	}
	for _, kind := range []string{"nil", "unknown", "name", "proof", "proof-scope", "path", "leaf"} {
		p := *f.fixture.packages["user"]
		proof := *p.Proof
		p.Proof, p.Skill = &proof, validated.skill
		scope := "user"
		candidate := &p
		switch kind {
		case "nil":
			candidate = nil
		case "unknown":
			scope = "outside"
		case "name":
			p.Name = "uap-kimi-native-a"
		case "proof":
			p.Proof = nil
		case "proof-scope":
			proof.scope = "a"
		case "path":
			p.Skill = "relative/SKILL.md"
		case "leaf":
			proof.authored = map[string]kimiProjectProofFile{}
		}
		if row, err := kimiProjectExpectedSkill(scope, candidate); err == nil || row != nil {
			t.Fatal("invalid expectation acquired native authority", kind)
		}
	}
}
