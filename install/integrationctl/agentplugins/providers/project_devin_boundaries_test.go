package providers_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/windsurf"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func TestProjectDevinAttestationCannotUpgradeConfiguration(t *testing.T) {
	fixture := newDevinProjectFixture(t, false)
	before := projectGeminiInventory(t, fixture.request.Plan.NativeRegistryRoot)
	fixture.request.ActivationComplete = true
	for _, prepared := range []bool{false, true} {
		if prepared {
			fixture.request.Plan.Components[0].Support = domain.SupportPrepared
		}
		_, err := fixture.activator(nativeconfig.New()).Activate(context.Background(), fixture.request)
		if err == nil || !strings.Contains(err.Error(), "cannot be established by activation attestation") {
			t.Fatalf("attestation bypassed preflight: %v", err)
		}
	}
	if _, err := windsurf.New().Activate(context.Background(), clients.Env{NativeConfig: nativeconfig.New()}, fixture.request); err == nil {
		t.Fatal("direct adapter bypassed preflight")
	}
	if !reflect.DeepEqual(before, projectGeminiInventory(t, fixture.request.Plan.NativeRegistryRoot)) {
		t.Fatal("attestation changed project files")
	}
}

func TestProjectDevinForeignDisabledMalformedAndWriterFailurePreserveNativeFiles(t *testing.T) {
	for _, failure := range []string{"disabled-collision", "malformed", "skill-collision", "writer-lock"} {
		t.Run(failure, func(t *testing.T) {
			fixture := newDevinProjectFixture(t, false)
			root := fixture.request.Plan.NativeRegistryRoot
			kernel := nativeconfig.New()
			switch failure {
			case "disabled-collision":
				projectGeminiWrite(t, filepath.Join(root, "mcp_config.json"), `{"mcpServers":{"docs":{"url":"https://foreign.test/keep","disabled":true}}}`)
			case "malformed":
				projectGeminiWrite(t, filepath.Join(root, "mcp_config.json"), `{"mcpServers":[]}`)
			case "writer-lock":
				kernel = nativeconfig.NewWithLockAcquirer(func(nativeconfig.Paths, nativeconfig.Codec) (func() error, error) {
					return nil, errors.New("inert Devin writer refusal")
				})
			case "skill-collision":
				projectGeminiWrite(t, filepath.Join(root, "skills", "docs", "SKILL.md"), "foreign same-name skill\n")
			}
			before, user, other := projectGeminiInventory(t, root), projectGeminiInventory(t, fixture.user), projectGeminiInventory(t, fixture.other)
			if _, err := fixture.activator(kernel).Activate(context.Background(), fixture.request); err == nil {
				t.Fatal("unsafe/failed native mutation succeeded")
			}
			if !reflect.DeepEqual(before, projectGeminiInventory(t, root)) || !reflect.DeepEqual(user, projectGeminiInventory(t, fixture.user)) || !reflect.DeepEqual(other, projectGeminiInventory(t, fixture.other)) {
				t.Fatal("failed install changed native files")
			}
		})
	}
}

func TestProjectDevinScopeRootAndBindingAuthorityRefuse(t *testing.T) {
	for _, mismatch := range []string{"user-registry", "user-context", "other-user", "omitted-user", "copied-project"} {
		t.Run(mismatch, func(t *testing.T) {
			fixture := newDevinProjectFixture(t, false)
			root := fixture.request.Plan.NativeRegistryRoot
			before := projectGeminiInventory(t, root)
			switch mismatch {
			case "user-registry":
				fixture.request.Plan.NativeRegistryRoot = fixture.user
			case "user-context":
				fixture.request.Plan.TargetContext.Scope = domain.ScopeUser
			case "omitted-user":
				fixture.request.Plan.TargetContext.UserConfigRoot = ""
			case "other-user":
				fixture.request.Plan.TargetContext.UserConfigRoot = fixture.other
			case "copied-project":
				fixture.request.Plan.TargetContext.ProjectRoot = fixture.other
				fixture.request.Plan.NativeRegistryRoot = filepath.Join(fixture.other, ".devin")
			}
			if _, err := fixture.activator(nativeconfig.New()).Activate(context.Background(), fixture.request); err == nil {
				t.Fatal("mismatched project authority admitted")
			}
			if !reflect.DeepEqual(before, projectGeminiInventory(t, root)) {
				t.Fatal("authority refusal changed original project")
			}
		})
	}
}

func TestProjectDevinContainedNativeRootAliasRefuses(t *testing.T) {
	fixture := newDevinProjectFixture(t, false)
	root := fixture.request.Plan.NativeRegistryRoot
	original := root + "-physical"
	if err := os.Rename(root, original); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(original, root); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	if _, err := fixture.activator(nativeconfig.New()).Activate(context.Background(), fixture.request); err == nil {
		t.Fatal("contained native root alias admitted")
	}
}
