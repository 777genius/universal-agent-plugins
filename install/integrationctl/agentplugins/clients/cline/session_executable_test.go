package cline

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func TestClineProjectSessionResolvesObservedCLIWithoutChangingFileOnlyAuthority(t *testing.T) {
	t.Parallel()
	want := filepath.Join(t.TempDir(), "bin", "cline")
	client := domain.DetectedClient{ClientID: domain.ClientCline, Status: domain.DetectionDetected,
		Surfaces: []domain.ClientSurface{{ID: "cline_cli", Detected: true}}}
	calls := 0
	env := clients.Env{LookupExecutable: func(name string) (string, error) {
		calls++
		if name != "cline" {
			t.Fatalf("unrelated executable lookup: %q", name)
		}
		return want, nil
	}}
	got, err := clineSessionExecutable(env, client, 1)
	if err != nil || got != want || calls != 1 || client.ExecutablePath != "" || client.Version != "" {
		t.Fatalf("session selection differs or changed installation authority: %q %v calls=%d %+v", got, err, calls, client)
	}
}

func TestClineSessionExistingCLIPathStillRequiresObservedSurface(t *testing.T) {
	t.Parallel()
	executable := filepath.Join(t.TempDir(), "cline")
	client := domain.DetectedClient{
		ClientID: domain.ClientCline, Status: domain.DetectionDetected,
		ExecutablePath: executable,
		Surfaces: []domain.ClientSurface{{ID: "cline_cli", Detected: true}},
	}
	got, err := clineSessionExecutable(clients.Env{}, client, 1)
	if err != nil || got != executable {
		t.Fatalf("observed CLI path was not retained: %q %v", got, err)
	}
}

func TestClineProjectSessionExecutableNegatives(t *testing.T) {
	t.Parallel()
	for _, mode := range []string{"editor-only", "editor-path", "absent-cli", "absent-cli-path", "foreign-client", "not-detected", "no-bindings", "nil-resolver", "lookup-error", "relative", "empty"} {
		t.Run(mode, func(t *testing.T) {
			client := domain.DetectedClient{ClientID: domain.ClientCline, Status: domain.DetectionDetected,
				Surfaces: []domain.ClientSurface{{ID: "cline_cli", Detected: true}}}
			bindings, calls := 1, 0
			env := clients.Env{LookupExecutable: func(string) (string, error) {
				calls++
				switch mode {
				case "lookup-error":
					return "", errors.New("missing CLI")
				case "relative":
					return "bin/cline", nil
				case "empty":
					return "", nil
				default:
					return filepath.Join(t.TempDir(), "cline"), nil
				}
			}}
			switch mode {
			case "editor-only":
				client.Surfaces = []domain.ClientSurface{{ID: "cline_vscode_extension", Detected: true}}
			case "absent-cli":
				client.Surfaces[0].Detected = false
			case "editor-path":
				client.Surfaces = []domain.ClientSurface{{ID: "cline_vscode_extension", Detected: true}}
				client.ExecutablePath = filepath.Join(t.TempDir(), "editor")
			case "absent-cli-path":
				client.Surfaces[0].Detected = false
				client.ExecutablePath = filepath.Join(t.TempDir(), "cline")
			case "foreign-client":
				client.ClientID = "foreign"
			case "not-detected":
				client.Status = domain.DetectionNotDetected
			case "no-bindings":
				bindings = 0
			case "nil-resolver":
				env.LookupExecutable = nil
			}
			if _, err := clineSessionExecutable(env, client, bindings); err == nil {
				t.Fatal("invalid session selection accepted")
			}
			if mode != "lookup-error" && mode != "relative" && mode != "empty" && calls != 0 {
				t.Fatal("unadmitted selection performed a PATH lookup")
			}
		})
	}
}
