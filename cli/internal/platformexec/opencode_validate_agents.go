package platformexec

func validateOpenCodeAgentFiles(root string, rels []string) []Diagnostic {
	return validateOpenCodeAgentMarkdownFiles(root, rels)
}

func validateOpenCodeDefaultAgent(root string, rel string) []Diagnostic {
	return validateOpenCodeDefaultAgentFile(root, rel)
}

func validateOpenCodeAgentFrontmatter(rel string, frontmatter map[string]any) []Diagnostic {
	return validateOpenCodeAgentFrontmatterFields(rel, frontmatter)
}

func validateOpenCodeAgentStringField(rel string, frontmatter map[string]any, field string) []Diagnostic {
	return validateOpenCodeAgentStringFrontmatter(rel, frontmatter, field)
}
