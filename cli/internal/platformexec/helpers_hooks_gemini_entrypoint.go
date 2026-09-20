package platformexec

import "strings"

func trimGeminiHookCommand(command, invocation string) (string, bool) {
	command = strings.TrimSpace(command)
	suffix := " " + strings.TrimSpace(invocation)
	if !strings.HasSuffix(command, suffix) {
		return "", false
	}
	entrypoint := strings.TrimSpace(strings.TrimSuffix(command, suffix))
	if entrypoint == "" {
		return "", false
	}
	return normalizeGeminiHookEntrypoint(entrypoint), true
}

func normalizeGeminiHookEntrypoint(entrypoint string) string {
	entrypoint = strings.TrimSpace(entrypoint)
	switch {
	case strings.HasPrefix(entrypoint, "${extensionPath}${/}"):
		rel := strings.TrimPrefix(entrypoint, "${extensionPath}${/}")
		rel = strings.ReplaceAll(rel, "${/}", "/")
		rel = strings.TrimPrefix(rel, "/")
		if rel == "" {
			return "./"
		}
		return "./" + rel
	case strings.HasPrefix(entrypoint, "${extensionPath}/"):
		rel := strings.TrimPrefix(entrypoint, "${extensionPath}/")
		rel = strings.TrimPrefix(rel, "/")
		if rel == "" {
			return "./"
		}
		return "./" + rel
	default:
		return entrypoint
	}
}

func geminiHookEntrypointForExtension(entrypoint string) string {
	entrypoint = strings.TrimSpace(entrypoint)
	if entrypoint == "" {
		return ""
	}
	if strings.HasPrefix(entrypoint, "${extensionPath}") {
		return entrypoint
	}
	if strings.HasPrefix(entrypoint, "./") {
		rel := strings.TrimPrefix(entrypoint, "./")
		rel = strings.TrimPrefix(rel, "/")
		if rel == "" {
			return "${extensionPath}"
		}
		return "${extensionPath}${/}" + strings.ReplaceAll(rel, "/", "${/}")
	}
	return entrypoint
}

func geminiHookCommandCandidates(entrypoint, invocation string) []string {
	candidates := []string{}
	seen := map[string]struct{}{}
	for _, base := range []string{
		strings.TrimSpace(entrypoint),
		geminiHookEntrypointForExtension(entrypoint),
	} {
		base = strings.TrimSpace(base)
		if base == "" {
			continue
		}
		command := base + " " + strings.TrimSpace(invocation)
		if _, ok := seen[command]; ok {
			continue
		}
		seen[command] = struct{}{}
		candidates = append(candidates, command)
	}
	return candidates
}
