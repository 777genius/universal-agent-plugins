package installer

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/all"
)

func TestProjectVSCodeDefaultSDKRefusesForeignNativeBeforePublication(t *testing.T) {
	for _, collision := range []string{"skill", "mcp", "malformed"} {
		t.Run(collision, func(t *testing.T) {
			f := newProjectFilesFixture(t, "vscode", "default")
			path, body := f.skill, "foreign skill"
			if collision != "skill" {
				path, body = f.config, `{"servers":{"docs":{"type":"http","url":"https://foreign.test/mcp"}}}`
				if collision == "malformed" {
					body = `{"servers":`
				}
			}
			deathWrite(t, path, body)
			other, before := f.untouched(t), f.state(t)
			out, err := f.execute(t, OpInstall)
			if err == nil || out.Mutated || f.state(t) != before || deathRead(t, path) != body {
				t.Fatalf("foreign workspace publication: %+v %v", out, err)
			}
			if _, err := os.Lstat(filepath.Join(f.cfg.StateRoot, "managed")); !os.IsNotExist(err) {
				t.Fatal("native refusal published a managed workspace package", err)
			}
			f.assertUntouched(t, other)
		})
	}
}

func TestProjectVSCodeRejectsAttestationAndCopilotSharedSkillAdoption(t *testing.T) {
	f := newProjectFilesFixture(t, "vscode", "disabled-with-runner")
	f.cfg.Registry = all.Project()
	f.reopen(t)
	other := f.untouched(t)
	f.req.ActivationComplete = true
	if out, err := f.execute(t, OpInstall); err == nil || out.Mutated || f.state(t) != "" {
		t.Fatalf("workspace attestation admitted before installation: %+v %v", out, err)
	}
	f.req.ActivationComplete = false
	f.apply(t, OpInstall)
	before, skill, config := f.state(t), deathRead(t, f.skill), deathRead(t, f.config)
	f.req.ActivationComplete = true
	if out, err := f.execute(t, OpRepair); err == nil || out.Mutated || f.state(t) != before || deathRead(t, f.skill) != skill || deathRead(t, f.config) != config {
		t.Fatalf("workspace attestation changed installed files: %+v %v", out, err)
	}
	f.req.ActivationComplete = false
	f.req.ClientID = "copilot"
	f.req.ClientConfigRoot = filepath.Join(f.base, "copilot-user")
	deathWrite(t, filepath.Join(f.req.ClientConfigRoot, "keep"), "original Copilot user")
	if out, err := f.execute(t, OpInstall); err == nil || out.Mutated || f.state(t) != before || deathRead(t, f.skill) != skill {
		t.Fatalf("Copilot adopted the same-byte VS Code skill: %+v %v", out, err)
	}
	f.assertUntouched(t, other)
}
