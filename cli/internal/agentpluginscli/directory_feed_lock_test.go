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

func TestSavedRegistryUpdateCannotChangeSigningLineage(t *testing.T) {
	loaded := loadedPackage{
		directory: &domain.DirectoryOrigin{FeedID: "modern-v1"},
		directorySelection: &domain.DirectorySelection{
			ProductID: "crm", DistributionID: "owner/crm", ReleaseSequence: 3,
		},
	}
	for _, tc := range []struct {
		name, previousFeed string
		wantError bool
	}{
		{"same modern feed", "modern-v1", false},
		{"legacy lock", "", true},
		{"another modern feed", "modern-v1-other", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			prior := &environment.LockedRegistrySource{
				FeedID: tc.previousFeed, ProductID: "crm", DistributionID: "owner/crm",
			}
			err := validateSavedRegistryUpdate(loaded, prior)
			if tc.wantError {
				if err == nil || !strings.Contains(err.Error(), "feed differs from locked source") {
					t.Fatalf("signed cross-feed update admitted: %v", err)
				}
			} else if err != nil {
				t.Fatalf("same-feed update refused: %v", err)
			}
		})
	}
}

func TestSavedRegistryUpdateRejectsOtherFeedBeforeDependencyDetection(t *testing.T) {
	for _, tc := range []struct{ name, lockedFeed, clientFeed string }{
		{"legacy lock modern client", "", "modern-v1"},
		{"modern lock legacy client", "modern-v1", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			client := &fixedDirectoryClient{}
			app := App{DirectoryClient: lockedFeedTestClient{
				fixedDirectoryClient: client, feedID: tc.clientFeed,
			}}
			snapshot := environmentSnapshot{Lock: environment.Lock{
				Packages: []environment.LockedPackage{{
					Alias: "crm", Source: environment.LockedSource{Registry: &environment.LockedRegistrySource{
						FeedID: tc.lockedFeed,
					}},
				}},
			}}
			_, changed, err := prepareSavedRegistryUpdate(context.Background(), app, true, snapshot, "crm", nil)
			if changed || err == nil || !strings.Contains(err.Error(), "feed differs from locked source") {
				t.Fatalf("cross-feed update crossed preflight: changed=%t err=%v", changed, err)
			}
			if client.calls != 0 {
				t.Fatalf("cross-feed update fetched Directory: calls=%d", client.calls)
			}
		})
	}
}

func TestLockedDirectoryFeedMismatchRefusesBeforeSourceCacheOrDisk(t *testing.T) {
	client := &fixedDirectoryClient{}
	app := App{DirectoryClient: lockedFeedTestClient{
		fixedDirectoryClient: client, feedID: "modern-v1",
	}}
	pin := environment.LockedPackage{Source: environment.LockedSource{
		Registry: &environment.LockedRegistrySource{FeedID: ""},
	}}
	_, err := app.acquireLockedSnapshot(context.Background(), pin)
	if err == nil || !strings.Contains(err.Error(), "feed differs from locked source") {
		t.Fatalf("cross-feed snapshot acquisition admitted: %v", err)
	}
	_, err = materializeLockedPackage(context.Background(), app, pin, false, nil)
	if err == nil || !strings.Contains(err.Error(), "feed differs from locked source") {
		t.Fatalf("cross-feed materialization admitted: %v", err)
	}
	if client.calls != 0 {
		t.Fatalf("cross-feed source guard fetched Directory: %d calls", client.calls)
	}
}

func TestLegacyRegistryLockRejectsUnknownSigningClient(t *testing.T) {
	pin := environment.LockedPackage{Source: environment.LockedSource{
		Registry: &environment.LockedRegistrySource{},
	}}
	if err := requireLockedDirectoryFeed(App{}, pin); err == nil || !strings.Contains(err.Error(), "signed Directory is unavailable") {
		t.Fatalf("missing trusted signing client accepted: %v", err)
	}
}

func TestCrossFeedRepairHydrationAndReplayRefuseBeforeCache(t *testing.T) {
	for _, tc := range []struct{ name, lockedFeed, clientFeed string }{
		{"legacy_locked_modern_client", "", "modern-v1"},
		{"modern_locked_legacy_client", "modern-v1", ""},
		{"different_modern_feeds", "modern-v1", "modern-next"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			client := &fixedDirectoryClient{}
			app := App{DirectoryClient: lockedFeedTestClient{
				fixedDirectoryClient: client, feedID: tc.clientFeed,
			}}
			pin := environment.LockedPackage{Source: environment.LockedSource{
				Registry: &environment.LockedRegistrySource{FeedID: tc.lockedFeed},
			}}
			_, hydrateErr := hydrateRecordedDependencyInventory(context.Background(), app, pin)
			if hydrateErr == nil || !strings.Contains(hydrateErr.Error(), "feed differs from locked source") {
				t.Fatalf("repair hydration crossed feed boundary: %v", hydrateErr)
			}
			_, replayErr := readReplayDependency(context.Background(), app, pin)
			if replayErr == nil || !strings.Contains(replayErr.Error(), "feed differs from locked source") {
				t.Fatalf("dependency replay crossed feed boundary: %v", replayErr)
			}
			if client.calls != 0 {
				t.Fatalf("cross-feed guard consulted Directory: %d calls", client.calls)
			}
		})
	}
}
