package installer

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/cursor"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func TestReserveIdentityRejectsInvalidProspectiveTargetWithoutWrites(t *testing.T) {
	base := t.TempDir()
	profileFile := filepath.Join(base, "profile.json")
	const original = `{"foreign":"retained"}`
	if err := os.WriteFile(profileFile, []byte(original), 0600); err != nil {
		t.Fatal(err)
	}
	registry, err := clients.NewRegistry(cursor.New())
	if err != nil {
		t.Fatal(err)
	}
	engine, err := New(Config{StateRoot: filepath.Join(base, "state"), TempRoot: filepath.Join(base, "staging"), Registry: registry})
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ name, root string }{{"relative root", "relative"}, {"profile is a file", profileFile}} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := engine.ReserveIdentity(IdentityRequest{
				ClientID: "cursor", InstallationID: "00000000-0000-4000-8000-000000000001",
				DeclaredName: "test-plugin", ClientConfigRoot: tc.root,
			})
			if err == nil {
				t.Fatal("invalid prospective target returned success")
			}
			for _, path := range []string{engine.cfg.StateRoot, engine.cfg.TempRoot} {
				if _, err := os.Lstat(path); !os.IsNotExist(err) {
					t.Fatalf("identity refusal created %s: %v", path, err)
				}
			}
			if body, err := os.ReadFile(profileFile); err != nil || string(body) != original {
				t.Fatalf("identity refusal changed client bytes: %q, %v", body, err)
			}
		})
	}
}

type identityTargetFailure struct {
	err   error
	calls int
}

func (*identityTargetFailure) ID() domain.ClientID { return domain.ClientCursor }
func (a *identityTargetFailure) TargetRoot(domain.DetectedClient, domain.PackageMode, string) (string, string, error) {
	a.calls++
	return "", "", a.err
}

func TestReserveIdentityPreservesTargetResolutionError(t *testing.T) {
	cause := errors.New("target layout unavailable")
	adapter := &identityTargetFailure{err: cause}
	registry, err := clients.NewRegistry(adapter)
	if err != nil {
		t.Fatal(err)
	}
	engine, err := New(Config{StateRoot: filepath.Join(t.TempDir(), "state"), Registry: registry})
	if err != nil {
		t.Fatal(err)
	}
	_, err = engine.ReserveIdentity(IdentityRequest{
		ClientID: "cursor", InstallationID: "00000000-0000-4000-8000-000000000001",
		DeclaredName: "test-plugin", ClientConfigRoot: filepath.Join(t.TempDir(), "profile"),
	})
	if !errors.Is(err, cause) || adapter.calls != 1 {
		t.Fatalf("target resolution error was lost: %v, calls=%d", err, adapter.calls)
	}
}

func TestReserveIdentityKeepsPartialAndRecordedReservations(t *testing.T) {
	const installationID = "00000000-0000-4000-8000-000000000001"
	request := IdentityRequest{ClientID: "cursor", InstallationID: installationID, DeclaredName: "test-plugin", ClientConfigRoot: "relative"}
	recorded := domain.ClientBinding{ClientID: "cursor", ClientBindingID: "recorded-binding", TargetLocator: "recorded-target", Scope: string(domain.ScopeUser)}
	for _, tc := range []struct {
		name   string
		change func(*IdentityRequest)
		state  domain.StateFileV2
		want   IdentityReservation
	}{
		{name: "no declared name", change: func(r *IdentityRequest) { r.DeclaredName = "" }, want: IdentityReservation{InstallationID: installationID, Scope: string(domain.ScopeUser)}},
		{name: "no client", change: func(r *IdentityRequest) { r.ClientID = "" }, want: IdentityReservation{InstallationID: installationID, Scope: string(domain.ScopeUser)}},
		{name: "no installation", change: func(r *IdentityRequest) { r.InstallationID = "" }, want: IdentityReservation{Scope: string(domain.ScopeUser)}},
		{name: "recorded binding", state: domain.StateFileV2{Installations: []domain.Installation{{InstallationID: installationID, Clients: map[string]domain.ClientBinding{recorded.ClientBindingID: recorded}}}}, want: IdentityReservation{InstallationID: installationID, BindingID: recorded.ClientBindingID, TargetPath: recorded.TargetLocator, Scope: recorded.Scope}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			adapter := &identityTargetFailure{err: errors.New("prospective target must not be resolved")}
			registry, err := clients.NewRegistry(adapter)
			if err != nil {
				t.Fatal(err)
			}
			engine, err := New(Config{StateRoot: filepath.Join(t.TempDir(), "state"), Registry: registry})
			if err != nil {
				t.Fatal(err)
			}
			engine.store = groupReportingStore{state: tc.state}
			req := request
			if tc.change != nil {
				tc.change(&req)
			}
			got, err := engine.ReserveIdentity(req)
			if err != nil || got != tc.want || adapter.calls != 0 {
				t.Fatalf("reservation changed: got=%+v want=%+v err=%v calls=%d", got, tc.want, err, adapter.calls)
			}
		})
	}
}
