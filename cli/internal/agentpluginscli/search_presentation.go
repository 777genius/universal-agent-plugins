package agentpluginscli

import (
	"strings"
)

// Human grouping is a presentation view, never an assertion that sources are
// interchangeable. JSON retains every record and its original identity.

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
