package agentpluginscli

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/environment"
)

type lockedFeedTestClient struct {
	*fixedDirectoryClient
	feedID string
}

func (client lockedFeedTestClient) SignedFeedID() string { return client.feedID }

func TestRegistryLockPinsAuthenticatedFeedIdentity(t *testing.T) {
	requirement := environment.Requirement{Alias: "crm", Registry: &environment.RegistrySource{
		Distribution: "owner/crm", Selector: environment.Selector{Kind: environment.SelectorRelease, Value: "2"},
	}}
	loaded := loadedPackage{
		directory: &domain.DirectoryOrigin{FeedID: "modern-v1"},
		directorySelection: &domain.DirectorySelection{
			ProductID: "crm", DistributionID: "owner/crm", ReleaseSequence: 2,
			Source: domain.DirectorySource{Repository: "owner/crm", Revision: strings.Repeat("a", 40)},
		},
	}
	pin, err := registryLockedPackage(requirement, loaded)
	if err != nil {
		t.Fatal(err)
	}
	if pin.Source.Registry == nil || pin.Source.Registry.FeedID != "modern-v1" {
		t.Fatalf("modern signed feed was lost from dependency lock: %+v", pin.Source.Registry)
	}
	encoded, err := json.Marshal(pin.Source.Registry)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(encoded), `"feed_id":"modern-v1"`) {
		t.Fatalf("modern feed was not serialized in the lock: %s", encoded)
	}

	// Pre-existing v1 locks have no field and must remain byte-compatible.
	loaded.directory.FeedID = ""
	legacy, err := registryLockedPackage(requirement, loaded)
	if err != nil {
		t.Fatal(err)
	}
	encoded, err = json.Marshal(legacy.Source.Registry)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), "feed_id") {
		t.Fatalf("legacy lock unexpectedly gained a feed field: %s", encoded)
	}
}

func TestLockedDirectoryRejectsCrossFeedBeforeAnyLookup(t *testing.T) {
	for _, tc := range []struct{ name, lockedFeed, clientFeed string }{
		{"legacy_lock_modern_client", "", "modern-v1"},
		{"modern_lock_legacy_client", "modern-v1", ""},
		{"different_modern_feeds", "modern-v1", "modern-v1-next"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fixture := newRolloutDirectoryFixture(t, []domain.ClientID{domain.ClientCodex}, nil)
			fixture.cli.app.DirectoryClient = lockedFeedTestClient{
				fixedDirectoryClient: fixture.directory, feedID: tc.clientFeed,
			}
			lock := environment.LockedPackage{Source: environment.LockedSource{
				Registry: &environment.LockedRegistrySource{FeedID: tc.lockedFeed},
			}}
			before := fixture.directory.calls
			_, err := verifyLockedDirectory(context.Background(), fixture.cli.app, loadedPackage{}, lock, nil)
			if err == nil || !strings.Contains(err.Error(), "feed differs from locked source") {
				t.Fatalf("cross-feed lock admitted or wrong error: %v", err)
			}
			if fixture.directory.calls != before {
				t.Fatalf("cross-feed mismatch reached source lookup: before=%d after=%d",
					before, fixture.directory.calls)
			}
		})
	}
}
