package agentpluginscli

import (
	"context"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/discoveryv1"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/spf13/cobra"
)

type searchCompatibility struct {
	Client         domain.ClientID                  `json:"client"`
	Authentication domain.AuthenticationRequirement `json:"authentication"`
	Delivery       string                           `json:"delivery"`
}

type searchResult struct {
	TrustState       string                  `json:"trust_state"`
	InstallSelector  string                  `json:"install_selector"`
	ProductID        string                  `json:"product_id"`
	DisplayName      string                  `json:"display_name"`
	Description      string                  `json:"description"`
	ManifestName     string                  `json:"manifest_name"`
	DistributionID   string                  `json:"distribution_id"`
	DistributionKind domain.DistributionKind `json:"distribution_kind"`
	Status           string                  `json:"status"`
	PackageVersion   string                  `json:"package_version"`
	Repository       string                  `json:"repository"`
	Revision         string                  `json:"revision"`
	PackagePath      string                  `json:"package_path"`
	SchemaURI        string                  `json:"schema_uri"`
	Components       []string                `json:"components"`
	Compatibility    []searchCompatibility   `json:"compatibility"`
	RuntimeReviewed  bool                    `json:"runtime_reviewed"`
	Stars            int                     `json:"stars"`
	matchScore       int
}

type searchResponse struct {
	Query                     string         `json:"query"`
	TrustFilter               string         `json:"trust_filter"`
	SnapshotSequence          uint64         `json:"snapshot_sequence"`
	SnapshotDigest            string         `json:"snapshot_digest"`
	DiscoveryStatus           string         `json:"discovery_status"`
	DiscoverySnapshotSequence uint64         `json:"discovery_snapshot_sequence,omitempty"`
	DiscoverySnapshotDigest   string         `json:"discovery_snapshot_digest,omitempty"`
	Results                   []searchResult `json:"results"`
	typoMatched               bool
}

type searchOptions struct {
	trust        string
	component    string
	client       string
	auth         string
	owner        string
	details      bool
	typoFallback bool
}

func newSearchCommand(app App, opts *options) *cobra.Command {
	search := &searchOptions{}
	command := &cobra.Command{
		Use:   "search <query>",
		Short: "Search signed Agent Plugin discovery metadata without downloading packages",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateCommonOptions(opts); err != nil {
				return err
			}
			search.typoFallback = opts.format != "json"
			response, err := searchReviewedDirectory(cmd.Context(), app, strings.TrimSpace(args[0]), *search)
			if err != nil {
				return err
			}
			if opts.format == "json" {
				return writeJSONOutput(cmd.OutOrStdout(), "search", response)
			}
			return writeHumanSearch(cmd.OutOrStdout(), response, *search)
		},
	}
	flags := command.Flags()
	flags.StringVar(&search.trust, "trust", "all", "trust state: all, reviewed, or unreviewed")
	flags.StringVar(&search.component, "component", "", "required component(s), comma-separated")
	flags.StringVar(&search.client, "client", "", "compatible client")
	flags.StringVar(&search.auth, "auth", "all", "authentication: all, required, not_required, or unknown")
	flags.StringVar(&search.owner, "owner", "", "source repository owner")
	flags.BoolVar(&search.details, "details", false, "show source, runtime, alternative sources, and unavailable records")
	return command
}

func searchReviewedDirectory(ctx context.Context, app App, query string, options searchOptions) (searchResponse, error) {
	if query == "" {
		return searchResponse{}, fmt.Errorf("search query is required")
	}
	trust := strings.ToLower(strings.TrimSpace(options.trust))
	if trust != "all" && trust != "reviewed" && trust != "unreviewed" {
		return searchResponse{}, fmt.Errorf("--trust must be all, reviewed, or unreviewed")
	}
	auth := strings.ToLower(strings.TrimSpace(options.auth))
	if auth != "all" && auth != string(domain.AuthenticationRequirementRequired) && auth != string(domain.AuthenticationRequirementNotRequired) && auth != string(domain.AuthenticationRequirementUnknown) {
		return searchResponse{}, fmt.Errorf("--auth must be all, required, not_required, or unknown")
	}
	components, err := parseSearchValues(options.component)
	if err != nil {
		return searchResponse{}, fmt.Errorf("--component: %w", err)
	}
	var selectedClient domain.ClientID
	if strings.TrimSpace(options.client) != "" {
		parsed, err := parseTargetOption(options.client)
		if err != nil || len(parsed) != 1 {
			return searchResponse{}, fmt.Errorf("--client must name exactly one supported client")
		}
		selectedClient = parsed[0]
	}
	state, err := app.StateStore.Load()
	if err != nil {
		return searchResponse{}, err
	}
	response := searchResponse{Query: query, TrustFilter: trust, DiscoveryStatus: "not_requested", Results: []searchResult{}}
	fallback := []searchResult{}
	products := map[string]domain.DirectoryProduct{}
	if trust != "unreviewed" {
		if app.DirectoryClient == nil {
			return searchResponse{}, fmt.Errorf("signed Directory dependencies are unavailable")
		}
		bundle, err := app.DirectoryClient.Load(ctx, installedDirectoryFloor(state))
		if err != nil {
			return searchResponse{}, fmt.Errorf("load signed Directory: %w", err)
		}
		response.SnapshotSequence, response.SnapshotDigest = bundle.Snapshot.Sequence, bundle.Digest
		products = make(map[string]domain.DirectoryProduct, len(bundle.Snapshot.Products))
		for _, product := range bundle.Snapshot.Products {
			products[product.ID] = product
		}
		for _, distribution := range bundle.Snapshot.Distributions {
			product, ok := products[distribution.ProductID]
			if !ok || !containsDirectoryValue(product.Distributions, distribution.ID) {
				continue
			}
			release, policy := currentSearchRelease(bundle.Snapshot, distribution)
			if release == nil || policy == nil {
				continue
			}
			result, ok := directorySearchResult(bundle.Snapshot, product, distribution, *release, *policy, query, options.owner, components, selectedClient, auth)
			if ok {
				result.matchScore = reviewedSearchRank(result.matchScore)
				response.Results = append(response.Results, result)
			} else if options.typoFallback && searchIdentityTypo(query, append([]string{product.ID, product.ManifestName}, product.Aliases...)...) {
				if candidate, matched := directorySearchResult(bundle.Snapshot, product, distribution, *release, *policy, product.ID, options.owner, components, selectedClient, auth); matched {
					fallback = append(fallback, candidate)
				}
			}
		}
	}
	if trust != "reviewed" {
		response.DiscoveryStatus = "unavailable"
		if app.DiscoveryClient == nil {
			if trust == "unreviewed" {
				return searchResponse{}, fmt.Errorf("signed Discovery Index dependencies are unavailable")
			}
			_, _ = fmt.Fprintln(app.errorOutput(), "WARNING [discovery_unavailable]: signed unreviewed search is unavailable; reviewed results remain usable")
		} else {
			discovery, err := app.DiscoveryClient.Load(ctx, 0)
			if err != nil {
				if trust == "unreviewed" {
					return searchResponse{}, fmt.Errorf("load signed Discovery Index: %w", err)
				}
				_, _ = fmt.Fprintf(app.errorOutput(), "WARNING [discovery_unavailable]: %v; reviewed results remain usable\n", err)
			} else {
				response.DiscoveryStatus = "available"
				response.DiscoverySnapshotSequence = discovery.Snapshot.Sequence
				response.DiscoverySnapshotDigest = discovery.Digest
				for _, record := range discovery.Search.Records {
					if result, ok := discoverySearchResult(record, query, options.owner, components, selectedClient, auth); ok {
						response.Results = append(response.Results, result)
					} else if options.typoFallback && searchIdentityTypo(query, record.Name) {
						if candidate, matched := discoverySearchResult(record, record.Name, options.owner, components, selectedClient, auth); matched {
							fallback = append(fallback, candidate)
						}
					}
				}
			}
		}
	}
	hasVisibleMatch := false
	for _, result := range response.Results {
		if result.Status == "available" || options.details {
			hasVisibleMatch = true
			break
		}
	}
	if options.typoFallback && !hasVisibleMatch {
		response.Results = append(response.Results, fallback...)
		response.typoMatched = len(fallback) > 0
	}
	sort.SliceStable(response.Results, func(i, j int) bool {
		left, right := response.Results[i], response.Results[j]
		if left.matchScore != right.matchScore {
			return left.matchScore < right.matchScore
		}
		if left.TrustState != right.TrustState {
			return left.TrustState == "reviewed"
		}
		leftDefault := left.TrustState == "reviewed" && left.DistributionID == products[left.ProductID].DefaultDistribution
		rightDefault := right.TrustState == "reviewed" && right.DistributionID == products[right.ProductID].DefaultDistribution
		if leftDefault != rightDefault {
			return leftDefault
		}
		if left.Stars != right.Stars {
			return left.Stars > right.Stars
		}
		if left.Repository != right.Repository {
			return left.Repository < right.Repository
		}
		if left.PackagePath != right.PackagePath {
			return left.PackagePath < right.PackagePath
		}
		return left.InstallSelector < right.InstallSelector
	})
	for index := range response.Results {
		response.Results[index].matchScore = 0
	}
	return response, nil
}

// Human grouping is a presentation view, never an assertion that sources are
// interchangeable. JSON retains every record and its original identity.
func writeHumanSearch(out io.Writer, response searchResponse, options searchOptions) error {
	groups := [][]searchResult{}
	positions := map[string]int{}
	for _, result := range response.Results {
		if result.Status != "available" && !options.details {
			continue
		}
		identity := strings.ToLower(strings.TrimSpace(result.ManifestName))
		if identity == "" {
			identity = strings.ToLower(result.ProductID)
		}
		if identity == "" {
			identity = result.InstallSelector
		}
		if index, ok := positions[identity]; ok {
			groups[index] = append(groups[index], result)
		} else {
			positions[identity] = len(groups)
			groups = append(groups, []searchResult{result})
		}
	}
	var output strings.Builder
	fmt.Fprintf(&output, "%d results for %q\n", len(groups), response.Query)
	if response.typoMatched && len(groups) > 0 {
		fmt.Fprintln(&output, "Showing close name matches (check the plugin name before adding).")
	}
	for _, group := range groups {
		// Preserve search relevance between groups, but prefer an available,
		// reviewed and upstream record as the group's primary source.
		sort.SliceStable(group, func(i, j int) bool {
			left, right := group[i], group[j]
			if (left.Status == "available") != (right.Status == "available") {
				return left.Status == "available"
			}
			leftShort := left.TrustState == "reviewed" && left.InstallSelector == left.ProductID
			rightShort := right.TrustState == "reviewed" && right.InstallSelector == right.ProductID
			if leftShort != rightShort {
				return leftShort
			}
			if left.TrustState != right.TrustState {
				return left.TrustState == "reviewed"
			}
			return left.DistributionKind == domain.DistributionUpstream && right.DistributionKind != domain.DistributionUpstream
		})
		result := group[0]
		fmt.Fprintf(&output, "  %s - %s [%s]\n", result.DisplayName, result.Description, result.TrustState)
		if result.Status == "available" {
			fmt.Fprintf(&output, "    npx universal-agent-plugins add %s", result.InstallSelector)
			if client := strings.ToLower(strings.TrimSpace(options.client)); client != "" {
				fmt.Fprintf(&output, " --target %s", client)
			}
			fmt.Fprintln(&output)
		} else {
			fmt.Fprintln(&output, "    install: unavailable at indexed source")
		}
		if options.details {
			runtime := "not reviewed"
			if result.RuntimeReviewed {
				runtime = "reviewed"
			}
			fmt.Fprintf(&output, "    source: %s\n    schema: %s; runtime: %s; status: %s\n", humanSearchSource(result), result.SchemaURI, runtime, result.Status)
			if len(group) > 1 {
				fmt.Fprintf(&output, "    Other sources (%d):\n", len(group)-1)
				for _, alternative := range group[1:] {
					fmt.Fprintf(&output, "      %s [%s, %s] - %s\n", alternative.InstallSelector, alternative.TrustState, alternative.Status, humanSearchSource(alternative))
				}
			}
		}
	}
	_, err := io.WriteString(out, output.String())
	return err
}

func humanSearchSource(result searchResult) string {
	source := result.Repository + "@" + result.Revision
	if result.PackagePath != "" {
		source += "//" + result.PackagePath
	}
	return source
}

// Limit fallback to one edit in a name of at least four characters. Short
// queries and repository selectors retain their precise matching semantics.
func searchIdentityTypo(query string, identities ...string) bool {
	query = strings.ToLower(strings.TrimSpace(query))
	if len(query) < 4 || strings.ContainsAny(query, " /:\t\n") {
		return false
	}
	for _, identity := range identities {
		left, right := []rune(query), []rune(strings.ToLower(identity))
		if len(left)-len(right) > 1 || len(right)-len(left) > 1 {
			continue
		}
		i, j, edits := 0, 0, 0
		for i < len(left) && j < len(right) {
			if left[i] == right[j] {
				i++
				j++
				continue
			}
			edits++
			if edits > 1 {
				break
			}
			if len(left) >= len(right) {
				i++
			}
			if len(right) >= len(left) {
				j++
			}
		}
		if edits+len(left)-i+len(right)-j == 1 {
			return true
		}
	}
	return false
}

func reviewedSearchRank(score int) int {
	if score <= 1 {
		return score
	}
	return score + 2
}

func discoverySearchResult(record discoveryv1.Record, query, owner string, components map[string]struct{}, client domain.ClientID, auth string) (searchResult, bool) {
	score, matched := discoveryMatchScore(query, record)
	if !matched || (strings.TrimSpace(owner) != "" && strings.ToLower(strings.TrimSpace(owner)) != record.Owner) {
		return searchResult{}, false
	}
	availableComponents := []string{}
	if record.Components.Extensions > 0 {
		availableComponents = append(availableComponents, "extensions")
	}
	if record.Components.MCP > 0 {
		availableComponents = append(availableComponents, "mcp")
	}
	if record.Components.Skills > 0 {
		availableComponents = append(availableComponents, "skills")
	}
	if !searchComponentsMatch(components, availableComponents) || (auth != "all" && auth != record.Authentication) {
		return searchResult{}, false
	}
	compatibility := make([]searchCompatibility, 0, len(record.CompatibleClients))
	for _, compatible := range record.CompatibleClients {
		clientID := domain.ClientID(compatible)
		if client != "" && clientID != client {
			continue
		}
		compatibility = append(compatibility, searchCompatibility{Client: clientID, Authentication: domain.AuthenticationRequirement(record.Authentication), Delivery: "portable-package"})
	}
	if client != "" && len(compatibility) == 0 {
		return searchResult{}, false
	}
	sort.Slice(compatibility, func(i, j int) bool { return compatibility[i].Client < compatibility[j].Client })
	version := ""
	if record.Version != nil {
		version = *record.Version
	}
	return searchResult{
		TrustState: "unreviewed", InstallSelector: record.Slug, ProductID: record.Name, DisplayName: record.Name,
		Description: record.Description, ManifestName: record.Name, Status: record.Availability, PackageVersion: version,
		Repository: record.Repository, Revision: record.Revision, PackagePath: record.PackagePath,
		SchemaURI: "https://agent-plugins.org/schemas/1.0.0/plugin.schema.json", Components: availableComponents,
		Compatibility: compatibility, RuntimeReviewed: false, Stars: record.Stars, matchScore: score,
	}, true
}

func discoveryMatchScore(query string, record discoveryv1.Record) (int, bool) {
	needle := strings.ToLower(strings.TrimSpace(query))
	identities := []string{record.Slug, record.Name, record.Repository}
	if record.PackagePath != "" {
		identities = append(identities, record.Repository+"//"+record.PackagePath)
	}
	for _, identity := range identities {
		if strings.ToLower(identity) == needle {
			return 2, true
		}
	}
	for _, identity := range identities {
		if strings.HasPrefix(strings.ToLower(identity), needle) {
			return 3, true
		}
	}
	for _, identity := range identities {
		if strings.Contains(strings.ToLower(identity), needle) {
			return 4, true
		}
	}
	if strings.Contains(strings.ToLower(record.Description), needle) {
		return 5, true
	}
	return 0, false
}

func currentSearchRelease(snapshot domain.DirectorySnapshot, distribution domain.DirectoryDistribution) (*domain.DirectoryRelease, *domain.DirectoryReleasePolicy) {
	policies := make(map[uint64]domain.DirectoryReleasePolicy, len(distribution.ReleasePolicies))
	for _, policy := range distribution.ReleasePolicies {
		policies[policy.ReleaseSequence] = policy
	}
	releases := append([]domain.DirectoryRelease(nil), distribution.Releases...)
	sort.SliceStable(releases, func(i, j int) bool { return releases[i].Sequence > releases[j].Sequence })
	for index := range releases {
		policy, ok := policies[releases[index].Sequence]
		if !ok || policy.Status == domain.ReleaseRevoked || releaseSearchRevoked(snapshot, distribution.ID, releases[index].Sequence) {
			continue
		}
		return &releases[index], &policy
	}
	return nil, nil
}

func directorySearchResult(snapshot domain.DirectorySnapshot, product domain.DirectoryProduct, distribution domain.DirectoryDistribution, release domain.DirectoryRelease, policy domain.DirectoryReleasePolicy, query, owner string, components map[string]struct{}, client domain.ClientID, auth string) (searchResult, bool) {
	score, matched := searchMatchScore(query, product, distribution, release)
	if !matched || !searchOwnerMatches(owner, distribution, release) || !searchComponentsMatch(components, release.Components) {
		return searchResult{}, false
	}
	compatibility := make([]searchCompatibility, 0, len(policy.Targets))
	for _, target := range policy.Targets {
		if client != "" && target.Client != client {
			continue
		}
		if auth != "all" && string(target.Authentication) != auth {
			continue
		}
		compatibility = append(compatibility, searchCompatibility{Client: target.Client, Authentication: target.Authentication, Delivery: target.Delivery})
	}
	if (client != "" || auth != "all") && len(compatibility) == 0 {
		return searchResult{}, false
	}
	sort.Slice(compatibility, func(i, j int) bool { return compatibility[i].Client < compatibility[j].Client })
	selector := distribution.ID
	if distribution.ID == product.DefaultDistribution && distribution.Status == domain.DistributionActive {
		selector = product.ID
	}
	status := "available"
	if distribution.Status != domain.DistributionActive || policy.Status != domain.ReleaseActive {
		status = "unavailable"
	}
	return searchResult{
		TrustState: "reviewed", InstallSelector: selector, ProductID: product.ID, DisplayName: product.DisplayName,
		Description: product.Description, ManifestName: product.ManifestName, DistributionID: distribution.ID,
		DistributionKind: distribution.Kind, Status: status, PackageVersion: release.PackageVersion,
		Repository: release.PackageSource.Repository, Revision: release.PackageSource.Revision, PackagePath: release.PackageSource.Path,
		SchemaURI: release.AgentPluginsSchema, Components: append([]string(nil), release.Components...), Compatibility: compatibility,
		RuntimeReviewed: searchHasRuntimeEvidence(snapshot, distribution.ID, release.Sequence, policy, client), matchScore: score,
	}, true
}

func searchMatchScore(query string, product domain.DirectoryProduct, distribution domain.DirectoryDistribution, release domain.DirectoryRelease) (int, bool) {
	needle := strings.ToLower(strings.TrimSpace(query))
	identities := []string{product.ID, product.ManifestName, distribution.ID, release.ManifestName}
	identities = append(identities, product.Aliases...)
	for _, identity := range identities {
		if strings.ToLower(identity) == needle {
			return 0, true
		}
	}
	for _, identity := range identities {
		if strings.HasPrefix(strings.ToLower(identity), needle) {
			return 1, true
		}
	}
	for _, identity := range identities {
		if strings.Contains(strings.ToLower(identity), needle) {
			return 2, true
		}
	}
	if strings.Contains(strings.ToLower(product.DisplayName+" "+product.Description), needle) {
		return 3, true
	}
	return 0, false
}

func searchOwnerMatches(owner string, distribution domain.DirectoryDistribution, release domain.DirectoryRelease) bool {
	owner = strings.ToLower(strings.TrimSpace(owner))
	if owner == "" {
		return true
	}
	for _, value := range []string{distribution.ID, release.PackageSource.Repository} {
		parts := strings.SplitN(strings.ToLower(value), "/", 2)
		if len(parts) == 2 && parts[0] == owner {
			return true
		}
	}
	return false
}

func searchComponentsMatch(required map[string]struct{}, available []string) bool {
	if len(required) == 0 {
		return true
	}
	seen := map[string]struct{}{}
	for _, value := range available {
		seen[strings.ToLower(strings.TrimSpace(value))] = struct{}{}
	}
	for value := range required {
		if _, ok := seen[value]; !ok {
			return false
		}
	}
	return true
}

func parseSearchValues(value string) (map[string]struct{}, error) {
	result := map[string]struct{}{}
	if strings.TrimSpace(value) == "" {
		return result, nil
	}
	for _, item := range strings.Split(value, ",") {
		item = strings.ToLower(strings.TrimSpace(item))
		if item == "" {
			return nil, fmt.Errorf("empty value")
		}
		result[item] = struct{}{}
	}
	return result, nil
}

func releaseSearchRevoked(snapshot domain.DirectorySnapshot, distribution string, sequence uint64) bool {
	for _, revocation := range snapshot.Revocations {
		if revocation.DistributionID == distribution && revocation.ReleaseSequence == sequence {
			return true
		}
	}
	return false
}

func searchHasRuntimeEvidence(snapshot domain.DirectorySnapshot, distribution string, sequence uint64, policy domain.DirectoryReleasePolicy, client domain.ClientID) bool {
	current := make(map[string]struct{}, len(policy.CurrentEvidence))
	for _, id := range policy.CurrentEvidence {
		current[id] = struct{}{}
	}
	for _, evidence := range snapshot.Evidence {
		_, active := current[evidence.ID]
		if active && evidence.DistributionID == distribution && evidence.ReleaseSequence == sequence &&
			(client == "" || evidence.Client == client) &&
			evidence.Level == "runtime" && evidence.Outcome == "passed" && evidence.HasTrustedEligibilityProvenanceAtSequence(snapshot.Sequence) {
			return true
		}
	}
	return false
}
