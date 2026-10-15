package installer

import (
	"context"
	"errors"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"testing"
)

func TestProjectedInstallRequiresPreparedOwnedUpdate(t *testing.T) {
	engine := &Engine{}
	authority := &domain.EnvironmentAuthority{EnvironmentID: "user-default", Alias: "demo", SourceKey: "github:acme/demo:", InstallationID: "i", ClientID: "codex"}
	update := &PreparedOperation{engine: engine, req: Request{Operation: OpUpdate, InstallationID: "i", EnvironmentAuthority: authority}, snapshot: domain.PackageSnapshot{TreeDigest: "desired"}}
	wanted := *authority
	wanted.ClientID = "claude"
	req := Request{Operation: OpInstall, InstallationID: "i", ClientID: "claude", EnvironmentAuthority: &wanted, AcquiredSnapshot: &domain.AcquiredSnapshot{TreeDigest: "other"}}
	if _, err := engine.PreviewInstallAfterUpdate(context.Background(), req, update); !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("wrong desired pin: %v", err)
	}
	req.AcquiredSnapshot.TreeDigest = "desired"
	wanted.Alias = "different"
	if _, err := engine.PreviewInstallAfterUpdate(context.Background(), req, update); !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("cross-alias projection: %v", err)
	}
	update.req.EnvironmentAuthority = nil
	if _, err := engine.PreviewInstallAfterUpdate(context.Background(), req, update); !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("unmanaged projection: %v", err)
	}
	previewEngine := *engine
	if _, err := engine.Apply(context.Background(), &PreparedOperation{engine: &previewEngine}, Decision{Confirmed: true}); !errors.Is(err, ErrInvalidHandle) {
		t.Fatalf("preview handle applied: %v", err)
	}
}
