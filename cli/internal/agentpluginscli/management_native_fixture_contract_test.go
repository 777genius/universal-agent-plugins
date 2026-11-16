package agentpluginscli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"

	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli/management"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	legacyports "github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

func TestManagementNativeFixtureUsesCanonicalExecutable(t *testing.T) {
	physical, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	alias := filepath.Join(t.TempDir(), "temp-alias")
	if err := os.Symlink(physical, alias); err != nil {
		if runtime.GOOS == "windows" {
			t.Skipf("symlink privilege unavailable: %v", err)
		}
		t.Fatal(err)
	}
	for _, key := range []string{"TMPDIR", "TEMP", "TMP"} {
		t.Setenv(key, alias)
	}
	// A fresh child test creates its TempDir through the alias, as macOS /var
	// and Windows short TEMP spellings do in the hosted management journeys.
	t.Run("aliased_temp_root", func(t *testing.T) {
		fixture, base := managementDependencySpecHarness(t, "", dependencyClientSpec{id: domain.ClientOpenCode, executable: true})
		detected, err := fixture.app.Detector.Detect(t.Context())
		if err != nil || len(detected) != 1 {
			t.Fatalf("fixture detection: %+v, %v", detected, err)
		}
		client := detected[0]
		captured, err := NormalizeManagementStartup(management.Startup{
			Scope: management.User, StateRoot: filepath.Dir(fixture.app.ManagedRoot), DependenciesRoot: fixture.app.DependenciesRoot,
			Targets: []management.StartupTarget{{Id: "selected", Client: string(client.ClientID), ConfigRoot: client.ConfigRoot, Executable: client.ExecutablePath}},
		})
		if err != nil {
			t.Fatal(err)
		}
		target := captured.Targets[0]
		skillPath := filepath.Join(target.ConfigRoot, "skills", "fixture-guide", "SKILL.md")
		if err := os.MkdirAll(filepath.Dir(skillPath), 0700); err != nil {
			t.Fatal(err)
		}
		dependencyWrite(t, filepath.Join(target.ConfigRoot, "opencode.json"), `{"selected":"fixture"}`)
		dependencyWrite(t, filepath.Join(target.ConfigRoot, "opencode.jsonc"), `{"retained":"native"}`)
		dependencyWrite(t, skillPath, "---\nname: fixture-guide\ndescription: Fixture guide\n---\nExact native content.\n")
		runner := newManagementNativeRunner(detected, base)
		// Later caller edits must not replace the captured executable/profile.
		detected[0].ExecutablePath = filepath.Join(t.TempDir(), "uncaptured-opencode")
		detected[0].ConfigRoot = filepath.Join(t.TempDir(), "uncaptured-profile")
		command := legacyports.Command{
			Argv: []string{target.Executable, "debug", "config"}, Dir: target.ConfigRoot,
			StdoutLimitBytes: 1 << 20,
		}
		out, err := runner.Run(t.Context(), command)
		if err != nil {
			t.Fatal("canonical executable missed its selected native responder", err)
		}
		var config map[string]string
		if err := json.Unmarshal(out.Stdout, &config); err != nil || !reflect.DeepEqual(config, map[string]string{"selected": "fixture", "retained": "native"}) {
			t.Fatalf("selected native config was not observed: %s, %v", out.Stdout, err)
		}
		command.Argv[2] = "skill"
		out, err = runner.Run(t.Context(), command)
		var skills []map[string]string
		if err != nil || json.Unmarshal(out.Stdout, &skills) != nil || !reflect.DeepEqual(skills, []map[string]string{{"name": "fixture-guide", "location": skillPath, "content": "Exact native content.\n"}}) {
			t.Fatalf("selected native Skill was not observed: %s, %v", out.Stdout, err)
		}
		command.Dir = t.TempDir()
		if _, err := runner.Run(t.Context(), command); err == nil {
			t.Fatal("uncaptured config root reached the selected responder")
		}
		command.Dir = target.ConfigRoot
		// The responder still requires exact captured identity; it does not match
		// another spelling at command dispatch or silently broaden its authority.
		// Leave one valid config so a fallback cannot refuse merely on ambiguity.
		if err := os.Remove(filepath.Join(target.ConfigRoot, "opencode.jsonc")); err != nil {
			t.Fatal(err)
		}
		relative, err := filepath.Rel(physical, target.Executable)
		if err != nil {
			t.Fatal(err)
		}
		command.Argv[0], command.Argv[2] = filepath.Join(alias, relative), "config"
		if command.Argv[0] == target.Executable {
			t.Fatal("test did not create distinct executable spellings")
		}
		if _, err := runner.Run(t.Context(), command); err == nil {
			t.Fatal("uncaptured executable spelling reached the selected responder")
		}
	})
}
