//go:build linux

package pluginkitairepo_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"testing"
	"time"
)

func copilotPostAddInertPhaseFixture(t *testing.T, mode string) *copilotProjectAttempt {
	t.Helper()
	f := copilotEmptyRegistryContractFixture(t)
	f.postAddRegistryEvidence()
	f.evidence["commands"] = slices.Clone(copilotPostAddRegistryCommands[:3])
	installer := "#!/bin/sh\nprintf '%s\\n' '{\"schema_version\":1,\"command\":\"add\",\"result\":\"success\",\"data\":{}}'\n"
	list := fmt.Sprintf("#!/bin/sh\n[ \"$#\" = 2 ] && [ \"$1\" = plugin ] && [ \"$2\" = list ] || exit 99\nprintf listed >%q\nprintf 'opaque inert registry\\n'\nprintf 'opaque diagnostic\\n' >&2\n", filepath.Join(f.base.Root, "list-executed"))
	switch mode {
	case "add-failed":
		installer += "exit 7\n"
	case "add-config":
		installer += "printf '{}\\n' >\"$HOME/.copilot/config.json\"\n"
	case "after-user-metadata":
		installer += "ln -s /absent \"$HOME/.copilot/inert-alias\"\n"
	case "list-failed":
		list += "exit 7\n"
	case "list-config":
		list += "printf '{}\\n' >\"$HOME/.copilot/config.json\"\n"
	case "list-pin":
		list += "printf changed >>\"$0\"\n"
	case "after-list-metadata":
		list += "ln -s /absent \"$HOME/.copilot/inert-alias\"\n"
	}
	projectNativeRuntimeWrite(t, f.installer, []byte(installer), 0700)
	projectNativeRuntimeWrite(t, f.client, []byte(list), 0700)
	f.immutable = map[string]string{f.installer: nativeSHA(t, f.installer), f.client: nativeSHA(t, f.client)}
	f.ctx, f.cancel = context.WithTimeout(context.Background(), 10*time.Second)
	t.Cleanup(f.cancel)
	return f
}

func TestCopilotPostAddRegistryActualInertPhasesPreserveOpaqueBytes(t *testing.T) {
	f := copilotPostAddInertPhaseFixture(t, "normal")
	baseline := bytes.Clone(f.originalConfigBaseline)
	f.postAddUserPhase(t)
	if status, ok := copilotPostAddMetadata(f.files["user-profile-metadata.json"], "add-user"); !ok || status != "observed" {
		t.Fatal("first phase metadata did not settle")
	}
	f.postAddListPhase(t)
	if !bytes.Equal(f.files["native-list.stdout.log"], []byte("opaque inert registry\n")) || !bytes.Equal(f.files["native-list.stderr.log"], []byte("opaque diagnostic\n")) {
		t.Fatal("opaque bytes changed")
	}
	if status, ok := copilotPostAddMetadata(f.files[copilotPostAddListMetadata], "native-list"); !ok || status != "observed" {
		t.Fatal("second phase metadata did not settle")
	}
	if !slices.Equal(f.evidence["commands"].([]string), copilotPostAddRegistryCommands) || !bytes.Equal(f.originalConfigBaseline, baseline) || f.protected[f.originalConfigPath] != copilotProjectConfigHash(baseline) {
		t.Fatal("prefix or original baseline changed")
	}
	f.check(t)
}

func copilotPostAddInertChild(t *testing.T, mode string) {
	t.Helper()
	f := copilotPostAddInertPhaseFixture(t, mode)
	t.Cleanup(func() {
		_, err := os.Lstat(filepath.Join(f.base.Root, "list-executed"))
		fmt.Printf("POST_ADD_CHILD stage=%s listed=%t user_meta=%t list_meta=%t\n", f.evidence["stage"], err == nil, len(f.files["user-profile-metadata.json"]) > 0, len(f.files[copilotPostAddListMetadata]) > 0)
	})
	switch mode {
	case "before-user":
		if err := os.Symlink("/absent", filepath.Join(f.base.Home, ".copilot", "inert-alias")); err != nil {
			t.Fatal(err)
		}
	case "before-list":
		f.postAddUserPhase(t)
		if err := os.Symlink("/absent", filepath.Join(f.base.Home, ".copilot", "inert-alias")); err != nil {
			t.Fatal(err)
		}
		f.postAddListPhase(t)
		fmt.Println("AFTER_POST_ADD_FATAL_GUARD")
		return
	}
	f.observePostAddRegistry(t)
	fmt.Println("AFTER_POST_ADD_FATAL_GUARD")
}

func TestCopilotPostAddRegistryOriginalFatalAndMetadataRefusalCannotContinue(t *testing.T) {
	const marker = "UAP_COPILOT_POST_ADD_INERT_CHILD"
	if mode := os.Getenv(marker); mode != "" {
		copilotPostAddInertChild(t, mode)
		return
	}
	for _, tc := range []struct {
		mode, message, stage string
		listed, listMeta     bool
	}{
		{"add-failed", "add-user process/capture/cleanup failed", "public-user", false, false},
		{"add-config", "original authority changed:", "public-user", false, false},
		{"before-user", "original pre-user metadata refused", "before-user", false, false},
		{"after-user-metadata", "user metadata refused; no native list started", "after-user-metadata", false, false},
		{"before-list", "original pre-list metadata refused", "before-list", false, true},
		{"list-failed", "native-list process/capture/cleanup failed", "native-list", true, true},
		{"list-config", "original authority changed:", "native-list", true, true},
		{"list-pin", "immutable identity changed:", "native-list", true, true},
		{"after-list-metadata", "native-list metadata refused", "after-list-metadata", true, true},
	} {
		copilotPostAddCheckInertChild(t, marker, tc.mode, tc.message, tc.stage, tc.listed, tc.listMeta)
	}
}

func copilotPostAddCheckInertChild(t *testing.T, marker, mode, message, stage string, listed, listMeta bool) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestCopilotPostAddRegistryOriginalFatalAndMetadataRefusalCannotContinue$", "-test.count=1")
	cmd.Env = append(os.Environ(), marker+"="+mode)
	output, err := cmd.CombinedOutput()
	var exited *exec.ExitError
	expected := fmt.Sprintf("POST_ADD_CHILD stage=%s listed=%t user_meta=true list_meta=%t", stage, listed, listMeta)
	if !errors.As(err, &exited) || exited.ExitCode() != 1 || !bytes.Contains(output, []byte(message)) || !bytes.Contains(output, []byte(expected)) || bytes.Contains(output, []byte("AFTER_POST_ADD_FATAL_GUARD")) {
		t.Fatalf("original inert guard did not preserve actual boundary %s: %v, %.2048s", mode, err, output)
	}
}
