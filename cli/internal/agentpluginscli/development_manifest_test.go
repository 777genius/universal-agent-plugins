package agentpluginscli

import (
 "reflect"
 "strings"
 "testing"
)

func TestSavedDevIntentPreservesForeignTextAndPromotesToNormal(t *testing.T) {
 original := []byte(`# project note
schema_version=1
[dependencies]
regular="^1.0" # keep
[dev-dependencies]
build="~2.0" # dev comment
[targets]
codex=["regular","build"]
`)
 requirements, err := readDependencyManifestBytes("uap.toml", original)
 if err != nil { t.Fatal(err) }
 if !requirements.Development["build"] { t.Fatalf("missing dev classification: %+v", requirements) }
 unchanged, err := editEnvironmentIntent(environmentSnapshot{Requirements:requirements, ManifestBytes:original}, requirements)
 if err != nil || string(unchanged) != string(original) { t.Fatalf("no-op changed bytes: %v %s", err, unchanged) }
 wanted := cloneSavedRequirements(requirements)
 delete(wanted.Development, "build")
 wanted.Development = nil
 edited, err := editEnvironmentIntent(environmentSnapshot{Requirements: requirements, ManifestBytes:original}, wanted)
 if err != nil { t.Fatal(err) }
 parsed, err := readDependencyManifestBytes("uap.toml", edited)
 if err != nil || !reflect.DeepEqual(parsed, wanted) { t.Fatalf("promotion changed intent: got %+v, want %+v: %v", parsed,wanted,err) }
 if !strings.Contains(string(edited), "# project note") || !strings.Contains(string(edited), "# keep") {
  t.Fatalf("promotion lost unrelated context: %s", edited)
 }
}

func TestSavedRemoveDevelopmentAliasClearsClassification(t *testing.T) {
 req, err := readDependencyManifestBytes("uap.toml", []byte(`schema_version=1
[dependencies]
regular="^1.0"
[dev-dependencies]
build="^2.0"
[targets]
codex=["regular","build"]
`))
 if err != nil { t.Fatal(err) }
 updated := removeSavedTargets(req, "build", []string{"codex"})
 if updated.Development["build"] || len(updated.Dependencies)!=1 || updated.Dependencies["regular"].Alias!="regular" {
  t.Fatalf("remove lost classification/neighbor: %+v", updated)
 }
 if err := updated.Validate(); err!=nil { t.Fatalf("post-remove invalid: %v",err) }
}
