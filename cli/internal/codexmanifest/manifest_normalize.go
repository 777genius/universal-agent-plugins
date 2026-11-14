package codexmanifest

import "strings"

func (a *Author) Normalize() {
	if a == nil {
		return
	}
	a.Name = strings.TrimSpace(a.Name)
	a.Email = strings.TrimSpace(a.Email)
	a.URL = strings.TrimSpace(a.URL)
}

func (a *Author) Empty() bool {
	if a == nil {
		return true
	}
	return strings.TrimSpace(a.Name) == "" &&
		strings.TrimSpace(a.Email) == "" &&
		strings.TrimSpace(a.URL) == ""
}

func (m *PackageMeta) Normalize() {
	if m == nil {
		return
	}
	if m.Author != nil {
		m.Author.Normalize()
		if m.Author.Empty() {
			m.Author = nil
		}
	}
	m.Homepage = strings.TrimSpace(m.Homepage)
	m.Repository = strings.TrimSpace(m.Repository)
	m.License = strings.TrimSpace(m.License)
	m.Keywords = normalizeStrings(m.Keywords)
}

func (m PackageMeta) Empty() bool {
	return m.Author == nil &&
		strings.TrimSpace(m.Homepage) == "" &&
		strings.TrimSpace(m.Repository) == "" &&
		strings.TrimSpace(m.License) == "" &&
		len(m.Keywords) == 0
}

func (m PackageMeta) Apply(doc map[string]any) {
	if doc == nil {
		return
	}
	if m.Author != nil && !m.Author.Empty() {
		author := map[string]any{}
		if strings.TrimSpace(m.Author.Name) != "" {
			author["name"] = m.Author.Name
		}
		if strings.TrimSpace(m.Author.Email) != "" {
			author["email"] = m.Author.Email
		}
		if strings.TrimSpace(m.Author.URL) != "" {
			author["url"] = m.Author.URL
		}
		if len(author) > 0 {
			doc["author"] = author
		}
	}
	if strings.TrimSpace(m.Homepage) != "" {
		doc["homepage"] = m.Homepage
	}
	if strings.TrimSpace(m.Repository) != "" {
		doc["repository"] = m.Repository
	}
	if strings.TrimSpace(m.License) != "" {
		doc["license"] = m.License
	}
	if len(m.Keywords) > 0 {
		doc["keywords"] = append([]string(nil), m.Keywords...)
	}
}

func normalizeStrings(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		out = append(out, value)
	}
	return out
}
