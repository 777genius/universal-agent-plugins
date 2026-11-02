package agentpluginscli

import (
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

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
