package main

import (
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func hermesColdTrustFixture(t *testing.T) (*hermesColdInputs, string, string) {
	t.Helper()
	base := projectJourneyTemp(t)
	root, project, other := filepath.Join(base, "selected"), filepath.Join(base, "project"), filepath.Join(base, "other-home")
	if err := os.Mkdir(project, 0700); err != nil {
		t.Fatal(err)
	}
	projectJourneyWrite(t, filepath.Join(root, "config.yaml"), hermesColdTrustFixtureConfig(""))
	projectJourneyWrite(t, filepath.Join(root, "skills", "foreign", "SKILL.md"), "retained foreign Skill\n")
	projectJourneyWrite(t, filepath.Join(root, "skills", "foreign", "assets", "reference.txt"), "retained nested asset\n")
	projectJourneyWrite(t, filepath.Join(root, "retained.txt"), "retained profile file\n")
	projectJourneyWrite(t, filepath.Join(other, "foreign.txt"), "different authority\n")
	input := &hermesColdInputs{protected: map[string]map[string]string{other: securityFixtureTree(t, other)}}
	input.rememberProfile(t, root)
	projectJourneyWrite(t, filepath.Join(root, "config.yaml"), hermesColdTrustFixtureConfig(project))
	return input, root, project
}

func hermesColdTrustFixtureConfig(project string) string {
	body := "theme: retained-selected-user\nmcp_servers: {}\n_config_version: 52\nretained_setting: original\nskills:\n  retained_option: false\n"
	if project != "" {
		body += fmt.Sprintf("  trusted_project_dirs:\n    - %q\n", project)
	}
	return body
}

func TestProjectHermesColdTrustAcceptsOnlyAuthorizedProfileChange(t *testing.T) {
	input, root, project := hermesColdTrustFixture(t)
	before := make(map[string]map[string]string, len(input.protected))
	for root, original := range input.protected {
		before[root] = maps.Clone(original)
	}
	// Native trust may replace config.yaml; every other old node retains its
	// held identity. A new initialization file outside the foreign subtree is OK.
	config := filepath.Join(root, "config.yaml")
	replacement := filepath.Join(root, "config.next")
	projectJourneyWrite(t, replacement, hermesColdTrustFixtureConfig(project))
	if err := os.Rename(replacement, config); err != nil {
		t.Fatal(err)
	}
	projectJourneyWrite(t, filepath.Join(root, "cache", "initialized.json"), "{}\n")
	if err := input.adoptProjectTrust(root, project); err != nil {
		t.Fatal("exact trust and native initialization were refused", err)
	}
	if !reflect.DeepEqual(input.protected[root], securityFixtureTree(t, root)) {
		t.Fatal("accepted profile did not retain the fully validated snapshot")
	}
	for authority, original := range before {
		if authority != root && !reflect.DeepEqual(original, input.protected[authority]) {
			t.Fatal("project trust renewed another authority")
		}
	}
	for path, original := range input.dataIdentities {
		current, err := os.Lstat(path)
		if err != nil || !os.SameFile(original, current) || original.Mode() != current.Mode() {
			t.Fatal("accepted trust lost an original held identity", err)
		}
	}
	accepted := maps.Clone(input.protected[root])
	if err := input.adoptProjectTrust(root, project); err == nil || !reflect.DeepEqual(accepted, input.protected[root]) {
		t.Fatal("the confirmed add's original snapshot could be renewed twice", err)
	}
}

func TestProjectHermesColdTrustRefusesForeignSkillDamageBeforeAdoption(t *testing.T) {
	for _, change := range []struct {
		name   string
		mutate func(*testing.T, string)
	}{
		{"skill bytes", func(t *testing.T, root string) {
			projectJourneyWrite(t, filepath.Join(root, "skills", "foreign", "SKILL.md"), "changed foreign Skill\n")
		}},
		{"nested asset bytes", func(t *testing.T, root string) {
			projectJourneyWrite(t, filepath.Join(root, "skills", "foreign", "assets", "reference.txt"), "changed retained asset\n")
		}},
		{"inserted file", func(t *testing.T, root string) {
			projectJourneyWrite(t, filepath.Join(root, "skills", "foreign", "unexpected.txt"), "new child\n")
		}},
		{"inserted subtree", func(t *testing.T, root string) {
			projectJourneyWrite(t, filepath.Join(root, "skills", "foreign", "new-directory", "child.txt"), "new nested child\n")
		}},
	} {
		t.Run(change.name, func(t *testing.T) {
			input, root, project := hermesColdTrustFixture(t)
			original := maps.Clone(input.protected[root])
			change.mutate(t, root)
			// The former theme/trust-only admission sees a valid exact trust
			// delta here. It must not hide damage by taking a new full baseline.
			if err := hermesColdTrustConfiguration(input.trustProfiles[root].config,
				[]byte(projectJourneyRead(t, filepath.Join(root, "config.yaml"))), project); err != nil {
				t.Fatal("control did not preserve the authorized config-only change", err)
			}
			if err := input.adoptProjectTrust(root, project); err == nil {
				t.Fatal("damaged foreign Skill was adopted as native initialization")
			}
			if !reflect.DeepEqual(original, input.protected[root]) || input.trustProfiles[root] == nil {
				t.Fatal("refusal changed or consumed the original profile baseline")
			}
		})
	}
}

func TestProjectHermesColdTrustPreservesFullYAMLAndOnePhysicalRoot(t *testing.T) {
	for _, change := range []struct {
		name   string
		mutate func(string, string) string
	}{
		{"old scalar changed", func(body, _ string) string {
			return strings.Replace(body, "retained_setting: original", "retained_setting: changed", 1)
		}},
		{"old nested setting removed", func(body, _ string) string { return strings.Replace(body, "  retained_option: false\n", "", 1) }},
		{"new default inserted", func(body, _ string) string { return body + "new_default: true\n" }},
		{"two physical root entries", func(body, project string) string { return body + fmt.Sprintf("    - %q\n", project) }},
		{"different root", func(body, project string) string {
			return strings.Replace(body, fmt.Sprintf("%q", project), fmt.Sprintf("%q", filepath.Dir(project)), 1)
		}},
		{"missing trust", func(_, _ string) string { return hermesColdTrustFixtureConfig("") }},
		{"trailing document", func(body, _ string) string { return body + "---\nextra: true\n" }},
		{"duplicate YAML key", func(body, _ string) string { return body + "theme: replacement\n" }},
	} {
		t.Run(change.name, func(t *testing.T) {
			input, root, project := hermesColdTrustFixture(t)
			original := maps.Clone(input.protected[root])
			projectJourneyWrite(t, filepath.Join(root, "config.yaml"), change.mutate(hermesColdTrustFixtureConfig(project), project))
			if err := input.adoptProjectTrust(root, project); err == nil {
				t.Fatal("unrelated config or project trust change was adopted")
			}
			if !reflect.DeepEqual(original, input.protected[root]) {
				t.Fatal("invalid config renewed the profile baseline")
			}
		})
	}
	t.Run("same physical root spelling", func(t *testing.T) {
		input, root, project := hermesColdTrustFixture(t)
		projectJourneyWrite(t, filepath.Join(root, "config.yaml"), hermesColdTrustFixtureConfig(project+string(filepath.Separator)+"."))
		if err := input.adoptProjectTrust(root, project); err != nil {
			t.Fatal("one physical selected project was refused", err)
		}
	})
}

func TestProjectHermesColdTrustPreservesRetainedModeAndIdentity(t *testing.T) {
	for _, replacement := range []string{"removed", "directory"} {
		t.Run(replacement, func(t *testing.T) {
			root, project := projectJourneyTemp(t), projectJourneyTemp(t)
			path := filepath.Join(root, "retained.txt")
			projectJourneyWrite(t, path, "retained bytes\n")
			projectJourneyWrite(t, filepath.Join(root, "config.yaml"), hermesColdTrustFixtureConfig(""))
			before, err := readHermesColdProfile(root)
			if err != nil {
				t.Fatal(err)
			}
			// These absence/type controls need no held-file replacement, and
			// therefore exercise the same real filesystem transition on Windows.
			if err := os.Remove(path); err != nil {
				t.Fatal(err)
			}
			if replacement == "directory" {
				if err := os.Mkdir(path, 0700); err != nil {
					t.Fatal(err)
				}
			}
			projectJourneyWrite(t, filepath.Join(root, "config.yaml"), hermesColdTrustFixtureConfig(project))
			after, err := readHermesColdProfile(root)
			if err != nil {
				t.Fatal(err)
			}
			if err := before.validate(after, project); err == nil || !strings.Contains(err.Error(), "removed or changed type/mode") {
				t.Fatal("retained node removal or type change was adopted", err)
			}
		})
	}
	t.Run("mode", func(t *testing.T) {
		input, root, project := hermesColdTrustFixture(t)
		path := filepath.Join(root, "retained.txt")
		original := input.trustProfiles[root].nodes["retained.txt"].info
		if err := os.Chmod(path, original.Mode().Perm()&^0222); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := os.Chmod(path, original.Mode().Perm()); err != nil {
				t.Error(err)
			}
		})
		if err := input.adoptProjectTrust(root, project); err == nil {
			t.Fatal("retained file mode change was adopted")
		}
	})
	t.Run("same bytes different identity", func(t *testing.T) {
		input, root, project := hermesColdTrustFixture(t)
		after, err := readHermesColdProfile(root)
		if err != nil {
			t.Fatal(err)
		}
		// Use metadata from a distinct real file so this control is portable
		// even where an open retained handle prevents replacing its pathname.
		replica := filepath.Join(projectJourneyTemp(t), "retained.txt")
		projectJourneyWrite(t, replica, "retained profile file\n")
		replacement, err := os.Lstat(replica)
		original := after.nodes["retained.txt"]
		if err != nil || os.SameFile(original.info, replacement) || original.info.Mode() != replacement.Mode() {
			t.Fatal("replacement identity control is not distinct and same-mode", err)
		}
		after.nodes["retained.txt"] = hermesColdProfileNode{info: replacement, digest: original.digest}
		if err := input.trustProfiles[root].validate(after, project); err == nil || !strings.Contains(err.Error(), "identity or content") {
			t.Fatal("identical retained bytes hid a different physical node", err)
		}
	})
}
