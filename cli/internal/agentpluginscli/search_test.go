package agentpluginscli

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/discoveryv1"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

type fixedDiscoveryClient struct {
	bundle discoveryv1.VerifiedBundle
	err    error
	calls  int
}

func (client *fixedDiscoveryClient) Load(context.Context, uint64) (discoveryv1.VerifiedBundle, error) {
	client.calls++
	return client.bundle, client.err
}

func discoverySearchBundle() discoveryv1.VerifiedBundle {
	version := "1.0.0"
	record := discoveryv1.Record{
		Slug: "discovery:upstream/demo//plugin", Name: "demo", Description: "unreviewed demo", Owner: "upstream",
		Repository: "upstream/demo", PackagePath: "plugin", Revision: strings.Repeat("a", 40), Version: &version,
		SchemaVersion: "1.0.0", Components: discoveryv1.Components{MCP: 1}, MCPTransports: []string{"streamable-http"},
		CompatibleClients: []string{"codex", "cursor"}, Authentication: "unknown", Status: "conformant_unreviewed",
		TreeDigest: "sha256:" + strings.Repeat("1", 64), ManifestDigest: "sha256:" + strings.Repeat("2", 64),
		RepositoryUpdatedAt: "2026-08-27T00:00:00Z", Availability: "available",
	}
	return discoveryv1.VerifiedBundle{
		Snapshot: discoveryv1.Snapshot{Sequence: 7}, Search: discoveryv1.Search{Sequence: 7, Records: []discoveryv1.Record{record}},
		Digest: "sha256:" + strings.Repeat("3", 64),
	}
}

func TestSearchReviewedDirectoryIsDeterministicReadOnlyAndFiltered(t *testing.T) {
	t.Parallel()
	fixture := newCLIFixture(t, nil)
	fixture.app.DirectoryClient = &fixedDirectoryClient{bundle: readModelBundle()}
	fixture.app.DiscoveryClient = &fixedDiscoveryClient{bundle: discoveryv1.VerifiedBundle{Snapshot: discoveryv1.Snapshot{Sequence: 1}, Search: discoveryv1.Search{Records: []discoveryv1.Record{}}, Digest: "sha256:" + strings.Repeat("3", 64)}}

	stdout, _, err := fixture.execute(false, "search", "demo", "--client", "cursor", "--owner", "owner", "--format", "json")
	if err != nil {
		t.Fatal(err)
	}
	assertVersionedJSON(t, stdout, "search")
	var envelope struct {
		Data searchResponse `json:"data"`
	}
	if err := json.Unmarshal([]byte(stdout), &envelope); err != nil {
		t.Fatal(err)
	}
	if len(envelope.Data.Results) == 0 || envelope.Data.Results[0].TrustState != "reviewed" || envelope.Data.Results[0].InstallSelector != "demo" {
		t.Fatalf("search results = %+v", envelope.Data.Results)
	}
	state, err := fixture.store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(state.Installations) != 0 {
		t.Fatalf("search mutated state: %+v", state)
	}

	repeat, _, err := fixture.execute(false, "search", "demo", "--client", "cursor", "--owner", "owner", "--format", "json")
	if err != nil || stdout != repeat {
		t.Fatalf("search is not deterministic: err=%v\nfirst=%s\nsecond=%s", err, stdout, repeat)
	}
	empty, _, err := fixture.execute(false, "search", "demo", "--trust", "unreviewed", "--format", "json")
	if err != nil || !strings.Contains(empty, `"results":[]`) {
		t.Fatalf("unreviewed filter = %q err=%v", empty, err)
	}
}

func TestSearchMergesReviewedAndSignedUnreviewedWithTrustRanking(t *testing.T) {
	t.Parallel()
	fixture := newCLIFixture(t, nil)
	fixture.app.DirectoryClient = &fixedDirectoryClient{bundle: readModelBundle()}
	fixture.app.DiscoveryClient = &fixedDiscoveryClient{bundle: discoverySearchBundle()}
	stdout, _, err := fixture.execute(false, "search", "demo", "--client", "cursor", "--format", "json")
	if err != nil {
		t.Fatal(err)
	}
	var envelope struct {
		Data searchResponse `json:"data"`
	}
	if err := json.Unmarshal([]byte(stdout), &envelope); err != nil {
		t.Fatal(err)
	}
	if envelope.Data.DiscoveryStatus != "available" || envelope.Data.DiscoverySnapshotSequence != 7 || len(envelope.Data.Results) < 2 {
		t.Fatalf("search response = %+v", envelope.Data)
	}
	unreviewedIndex := -1
	for index, result := range envelope.Data.Results {
		if result.TrustState == "unreviewed" {
			unreviewedIndex = index
			break
		}
	}
	if unreviewedIndex < 1 || envelope.Data.Results[0].TrustState != "reviewed" ||
		envelope.Data.Results[unreviewedIndex].InstallSelector != "discovery:upstream/demo//plugin" {
		t.Fatalf("ranked results = %+v", envelope.Data.Results)
	}
	unreviewed, _, err := fixture.execute(false, "search", "demo", "--trust", "unreviewed", "--owner", "upstream", "--component", "mcp", "--auth", "unknown", "--format", "json")
	if err != nil || !strings.Contains(unreviewed, `"trust_state":"unreviewed"`) || strings.Contains(unreviewed, `"trust_state":"reviewed"`) {
		t.Fatalf("unreviewed search = %q err=%v", unreviewed, err)
	}
}

func TestHumanSearchShowsExactProvenanceTrustAndRunnableInstallCommand(t *testing.T) {
	t.Parallel()
	fixture := newCLIFixture(t, nil)
	fixture.app.DirectoryClient = &fixedDirectoryClient{bundle: readModelBundle()}
	fixture.app.DiscoveryClient = &fixedDiscoveryClient{bundle: discoverySearchBundle()}
	stdout, _, err := fixture.execute(false, "search", "upstream/demo", "--trust", "unreviewed", "--client", "cursor", "--details")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"[unreviewed]",
		"source: upstream/demo@" + strings.Repeat("a", 40) + "//plugin",
		"schema: https://agent-plugins.org/schemas/1.0.0/plugin.schema.json; runtime: not reviewed",
		"npx universal-agent-plugins add discovery:upstream/demo//plugin --target cursor",
	} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("human search omitted %q:\n%s", want, stdout)
		}
	}
}

func TestHumanSearchOmitsTargetAndTechnicalDetailsWithoutClient(t *testing.T) {
	t.Parallel()
	fixture := newCLIFixture(t, nil)
	fixture.app.DirectoryClient = &fixedDirectoryClient{bundle: readModelBundle()}
	fixture.app.DiscoveryClient = &fixedDiscoveryClient{bundle: discoverySearchBundle()}
	stdout, _, err := fixture.execute(false, "search", "upstream/demo", "--trust", "unreviewed")
	if err != nil {
		t.Fatal(err)
	}
	want := "npx universal-agent-plugins add discovery:upstream/demo//plugin\n"
	if !strings.Contains(stdout, want) || strings.Contains(stdout, "--target") || strings.Contains(stdout, "source:") || strings.Contains(stdout, "runtime:") {
		t.Fatalf("human search is not compact:\n%s", stdout)
	}
}

func TestHumanSearchDoesNotOfferInstallForUnavailablePackage(t *testing.T) {
	t.Parallel()
	fixture := newCLIFixture(t, nil)
	fixture.app.DirectoryClient = &fixedDirectoryClient{bundle: readModelBundle()}
	bundle := discoverySearchBundle()
	bundle.Search.Records[0].Availability = "unavailable"
	fixture.app.DiscoveryClient = &fixedDiscoveryClient{bundle: bundle}
	stdout, _, err := fixture.execute(false, "search", "upstream/demo", "--trust", "unreviewed", "--client", "cursor")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stdout, "0 results") || strings.Contains(stdout, "npx universal-agent-plugins") {
		t.Fatalf("human search offered installation for an unavailable package:\n%s", stdout)
	}
	details, _, err := fixture.execute(false, "search", "upstream/demo", "--trust", "unreviewed", "--details")
	if err != nil || !strings.Contains(details, "install: unavailable at indexed source") || strings.Contains(details, "npx universal-agent-plugins") {
		t.Fatalf("unavailable details = %q, err = %v", details, err)
	}
	jsonOutput, _, err := fixture.execute(false, "search", "upstream/demo", "--trust", "unreviewed", "--format", "json")
	if err != nil || !strings.Contains(jsonOutput, `"status":"unavailable"`) {
		t.Fatalf("JSON lost unavailable result: %q, err = %v", jsonOutput, err)
	}
}

func TestHumanSearchGroupsSourcesWhileJSONRetainsRecords(t *testing.T) {
	t.Parallel()
	fixture := newCLIFixture(t, nil)
	fixture.app.DirectoryClient = &fixedDirectoryClient{bundle: readModelBundle()}
	fixture.app.DiscoveryClient = &fixedDiscoveryClient{bundle: discoverySearchBundle()}
	compact, _, err := fixture.execute(false, "search", "demo")
	if err != nil || !strings.HasPrefix(compact, "1 results") || strings.Count(compact, "npx universal-agent-plugins add ") != 1 || !strings.Contains(compact, "add demo\n") {
		t.Fatalf("compact grouping = %q, err = %v", compact, err)
	}
	details, _, err := fixture.execute(false, "search", "demo", "--details")
	if err != nil || !strings.Contains(details, "Other sources (") || !strings.Contains(details, "discovery:upstream/demo//plugin") {
		t.Fatalf("details = %q, err = %v", details, err)
	}
	first, _, err := fixture.execute(false, "search", "demo", "--format", "json")
	if err != nil {
		t.Fatal(err)
	}
	second, _, err := fixture.execute(false, "search", "demo", "--format", "json", "--details")
	if err != nil || first != second || !strings.Contains(first, "discovery:upstream/demo//plugin") {
		t.Fatalf("JSON changed with presentation flag: %q vs %q, err=%v", first, second, err)
	}
}

func TestHumanSearchTypoFallbackRespectsFiltersAndJSON(t *testing.T) {
	t.Parallel()
	fixture := newCLIFixture(t, nil)
	fixture.app.DirectoryClient = &fixedDirectoryClient{bundle: readModelBundle()}
	bundle := discoverySearchBundle()
	bundle.Search.Records[0].Name = "context7"
	bundle.Search.Records[0].Slug = "discovery:upstream/context7//plugin"
	fixture.app.DiscoveryClient = &fixedDiscoveryClient{bundle: bundle}
	stdout, _, err := fixture.execute(false, "search", "contex7", "--trust", "unreviewed")
	if err != nil || !strings.Contains(stdout, "context7") || !strings.Contains(stdout, "close name matches") {
		t.Fatalf("typo fallback = %q, err=%v", stdout, err)
	}
	for _, filters := range [][]string{{"--owner", "other"}, {"--component", "skills"}, {"--client", "kiro"}, {"--auth", "required"}} {
		args := append([]string{"search", "contex7", "--trust", "unreviewed"}, filters...)
		output, _, err := fixture.execute(false, args...)
		if err != nil || !strings.HasPrefix(output, "0 results") {
			t.Fatalf("filters %v: %q err=%v", filters, output, err)
		}
	}
	jsonOutput, _, err := fixture.execute(false, "search", "contex7", "--trust", "unreviewed", "--format", "json")
	if err != nil || !strings.Contains(jsonOutput, `"results":[]`) {
		t.Fatalf("JSON typo behavior changed: %q err=%v", jsonOutput, err)
	}
}

func TestSearchIdentityTypoIsBounded(t *testing.T) {
	for _, test := range []struct {
		query string
		want  bool
	}{{"contex7", true}, {"contextt7", true}, {"contexx7", true}, {"context7", false}, {"ctx", false}, {"cntex", false}, {"owner/contex7", false}} {
		if got := searchIdentityTypo(test.query, "context7"); got != test.want {
			t.Errorf("%q = %v, want %v", test.query, got, test.want)
		}
	}
}

func TestHumanSearchPrimarySourcePreference(t *testing.T) {
	t.Parallel()
	base := searchResult{ManifestName: "context7", ProductID: "context7", DisplayName: "Context7", TrustState: "unreviewed", Status: "available", InstallSelector: "copy"}
	community := base
	community.TrustState, community.InstallSelector = "reviewed", "context7"
	upstream := community
	upstream.DistributionKind, upstream.InstallSelector = domain.DistributionUpstream, "upstream"
	unavailable := upstream
	unavailable.Status, unavailable.InstallSelector = "unavailable", "unavailable"
	var output strings.Builder
	if err := writeHumanSearch(&output, searchResponse{Query: "context7", Results: []searchResult{base, unavailable, community, upstream}}, searchOptions{details: true}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "add context7\n") || strings.Count(output.String(), "npx universal-agent-plugins") != 1 || !strings.Contains(output.String(), "Other sources (3)") {
		t.Fatalf("primary preference: %s", output.String())
	}
}

func TestHumanSearchReviewedTypoAndExactMatchPrecedence(t *testing.T) {
	t.Parallel()
	fixture := newCLIFixture(t, nil)
	fixture.app.DirectoryClient = &fixedDirectoryClient{bundle: readModelBundle()}
	fixture.app.DiscoveryClient = &fixedDiscoveryClient{bundle: discoverySearchBundle()}
	output, _, err := fixture.execute(false, "search", "demoo", "--trust", "reviewed")
	if err != nil || !strings.Contains(output, "add demo\n") || !strings.Contains(output, "close name matches") {
		t.Fatalf("reviewed typo: %q err=%v", output, err)
	}
	bundle := discoverySearchBundle()
	bundle.Search.Records[0].Name = "demoo"
	fixture.app.DiscoveryClient = &fixedDiscoveryClient{bundle: bundle}
	output, _, err = fixture.execute(false, "search", "demoo")
	if err != nil || strings.Contains(output, "close name matches") || strings.Contains(output, "add demo\n") || !strings.HasPrefix(output, "1 results") {
		t.Fatalf("exact match precedence: %q err=%v", output, err)
	}
}

func TestSearchKeepsReviewedResultsWhenDiscoveryIsUnavailable(t *testing.T) {
	t.Parallel()
	fixture := newCLIFixture(t, nil)
	fixture.app.DirectoryClient = &fixedDirectoryClient{bundle: readModelBundle()}
	fixture.app.DiscoveryClient = &fixedDiscoveryClient{err: errors.New("offline")}
	stdout, stderr, err := fixture.execute(false, "search", "demo", "--format", "json")
	if err != nil || !strings.Contains(stdout, `"discovery_status":"unavailable"`) || !strings.Contains(stdout, `"trust_state":"reviewed"`) ||
		!strings.Contains(stderr, "discovery_unavailable") {
		t.Fatalf("graceful search = stdout:%q stderr:%q err:%v", stdout, stderr, err)
	}
}

func TestSearchUsesStarsOnlyAfterTrustAndTextRelevance(t *testing.T) {
	t.Parallel()
	fixture := newCLIFixture(t, nil)
	fixture.app.DirectoryClient = &fixedDirectoryClient{bundle: readModelBundle()}
	bundle := discoverySearchBundle()
	popular := bundle.Search.Records[0]
	popular.Slug = "discovery:zeta/popular//plugin"
	popular.Repository = "zeta/popular"
	popular.Owner = "zeta"
	popular.Stars = 100
	quiet := popular
	quiet.Slug = "discovery:alpha/quiet//plugin"
	quiet.Repository = "alpha/quiet"
	quiet.Owner = "alpha"
	quiet.Stars = 1
	bundle.Search.Records = []discoveryv1.Record{quiet, popular}
	fixture.app.DiscoveryClient = &fixedDiscoveryClient{bundle: bundle}

	stdout, _, err := fixture.execute(false, "search", "demo", "--trust", "unreviewed", "--format", "json")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Index(stdout, popular.Slug) > strings.Index(stdout, quiet.Slug) || !strings.Contains(stdout, `"stars":100`) {
		t.Fatalf("stars were not the final deterministic tie-breaker:\n%s", stdout)
	}
}

func TestSearchRuntimeReviewIsScopedToSelectedClient(t *testing.T) {
	t.Parallel()
	bundle := readModelBundle()
	distribution := bundle.Snapshot.Distributions[0]
	policy := distribution.ReleasePolicies[1]
	if !searchHasRuntimeEvidence(bundle.Snapshot, distribution.ID, 2, policy, "") ||
		!searchHasRuntimeEvidence(bundle.Snapshot, distribution.ID, 2, policy, "cursor") ||
		searchHasRuntimeEvidence(bundle.Snapshot, distribution.ID, 2, policy, "kiro") {
		t.Fatal("runtime review leaked across client identities")
	}
}
