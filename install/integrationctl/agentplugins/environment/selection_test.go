package environment

import (
	"context"
	"testing"
)

func TestSelectedSyncAndAttestation(t *testing.T) {
	req := Requirements{SchemaVersion: 1, Dependencies: map[string]Requirement{"demo": {Alias: "demo", Git: &GitSource{Repository: "acme/demo", Selector: Selector{Kind: SelectorRef, Value: "main"}}}}, Targets: map[string][]string{"codex": {"demo"}}}
	c := Coordinator{Resolver: &countingResolver{}}
	lock, err := c.Resolve(context.Background(), req, Lock{}, "")
	if err != nil {
		t.Fatal(err)
	}
	pkg := lock.Packages[0]
	observed := []BindingObservation{{Alias: "demo", ClientID: "codex", Managed: true, SourceKey: pkg.SourceKey, PackageDigest: pkg.TreeDigest, Revision: pkg.Source.Git.Commit}, {Alias: "other", ClientID: "claude", Managed: true, Drift: true}}
	plan, err := c.PrepareSyncFor(req, lock, observed, []string{"demo"}, &Attestation{Alias: "demo", ClientID: "codex", AuthComplete: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Bindings) != 1 || plan.Bindings[0].Action != ActionUpdate || !plan.Bindings[0].AuthComplete {
		t.Fatalf("%+v", plan)
	}
	if _, err = c.PrepareSyncFor(req, lock, observed, nil, nil); err == nil {
		t.Fatal("empty selection accepted")
	}
	plan, err = c.PrepareSyncFor(req, lock, observed, []string{"demo"}, nil)
	if err != nil || plan.Bindings[0].Action != ActionUnchanged {
		t.Fatalf("%+v %v", plan, err)
	}
	for _, bad := range []BindingObservation{
		{Alias: "demo", ClientID: "codex", Managed: true, SourceKey: pkg.SourceKey, PackageDigest: pkg.TreeDigest, Revision: "old"},
		{Alias: "demo", ClientID: "codex", Managed: true, SourceKey: pkg.SourceKey, PackageDigest: "old", Revision: pkg.Source.Git.Commit},
		{Alias: "demo", ClientID: "codex", SourceKey: pkg.SourceKey},
	} {
		if _, err = c.PrepareSyncFor(req, lock, []BindingObservation{bad}, []string{"demo"}, &Attestation{Alias: "demo", ClientID: "codex", AuthComplete: true}); err == nil {
			t.Fatal("attested unmatched pin")
		}
	}

	empty := Requirements{SchemaVersion: 1}
	digest, _ := ManifestDigest(empty)
	plan, err = c.PrepareSyncFor(empty, Lock{LockVersion: 1, ManifestDigest: digest}, observed[:1], []string{"demo"}, nil)
	if err != nil || len(plan.Bindings) != 1 || plan.Bindings[0].Action != ActionRemove {
		t.Fatalf("receipt removal: %+v %v", plan, err)
	}
	if _, err = c.PrepareSyncFor(empty, Lock{LockVersion: 1, ManifestDigest: digest}, observed[:1], []string{"demo"}, &Attestation{Alias: "demo", ClientID: "codex", AuthComplete: true}); err == nil {
		t.Fatal("removal attestation accepted")
	}
}

func TestSelectedSyncTargetsKeepOtherIntentAndBindings(t *testing.T) {
	req := Requirements{SchemaVersion: 1, Dependencies: map[string]Requirement{"demo": {Alias: "demo", Git: &GitSource{Repository: "acme/demo", Selector: Selector{Kind: SelectorRef, Value: "main"}}}}, Targets: map[string][]string{"codex": {"demo"}, "claude": {"demo"}}}
	c := Coordinator{Resolver: &countingResolver{}}
	lock, err := c.Resolve(context.Background(), req, Lock{}, "")
	if err != nil {
		t.Fatal(err)
	}
	// Unselected drift must neither block the selected install nor become removal.
	observed := []BindingObservation{{Alias: "demo", ClientID: "claude", Managed: true, Drift: true, SourceKey: lock.Packages[0].SourceKey}}
	plan, err := c.PrepareSyncForTargets(req, lock, observed, []string{"demo"}, nil, []string{"codex"})
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Bindings) != 1 || plan.Bindings[0].ClientID != "codex" || plan.Bindings[0].Action != ActionInstall {
		t.Fatalf("unexpected actions: %+v", plan.Bindings)
	}
	if len(lock.Packages[0].Targets) != 2 || len(req.Targets) != 2 {
		t.Fatal("saved targets mutated")
	}
	if _, err = c.PrepareSyncFor(req, lock, observed, []string{"demo"}, nil); err == nil {
		t.Fatal("alias-wide sync ignored saved drift")
	}
	observed[0].SourceKey = "different-source"
	if _, err = c.PrepareSyncForTargets(req, lock, observed, []string{"demo"}, nil, []string{"codex"}); err == nil {
		t.Fatal("selected sync ignored global source identity conflict")
	}
}
