package agentpluginscli

import (
	"context"
	"fmt"
	"sort"
	"strings"

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
			app, scopeErr := app.forOptions(opts)
			if scopeErr != nil {
				return scopeErr
			}
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
