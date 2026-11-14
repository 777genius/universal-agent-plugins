package pluginmodel

import (
	"slices"
	"strings"
)

func NormalizeManifest(m *Manifest) {
	m.APIVersion = strings.TrimSpace(m.APIVersion)
	if m.APIVersion == "" {
		m.APIVersion = APIVersionV1
	}
	m.Name = strings.TrimSpace(m.Name)
	m.Version = strings.TrimSpace(m.Version)
	m.Description = strings.TrimSpace(m.Description)
	if m.Author != nil {
		m.Author.Name = strings.TrimSpace(m.Author.Name)
		m.Author.Email = strings.TrimSpace(m.Author.Email)
		m.Author.URL = strings.TrimSpace(m.Author.URL)
		if m.Author.Name == "" && m.Author.Email == "" && m.Author.URL == "" {
			m.Author = nil
		}
	}
	m.Homepage = strings.TrimSpace(m.Homepage)
	m.Repository = strings.TrimSpace(m.Repository)
	m.License = strings.TrimSpace(m.License)
	m.Keywords = normalizeStrings(m.Keywords)
	for i, target := range m.Targets {
		m.Targets[i] = NormalizeTarget(target)
	}
	slices.Sort(m.Targets)
	m.Targets = slices.Compact(m.Targets)
}

func NormalizeLauncher(l *Launcher) {
	l.Runtime = NormalizeRuntime(l.Runtime)
	l.Entrypoint = strings.TrimSpace(l.Entrypoint)
}

func NormalizeTarget(target string) string {
	return strings.ToLower(strings.TrimSpace(target))
}

func NormalizeRuntime(runtime string) string {
	return strings.ToLower(strings.TrimSpace(runtime))
}

func normalizeStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		out = append(out, value)
	}
	slices.Sort(out)
	out = slices.Compact(out)
	if len(out) == 0 {
		return nil
	}
	return out
}
