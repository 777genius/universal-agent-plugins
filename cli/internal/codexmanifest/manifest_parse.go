package codexmanifest

func ParseInterfaceDoc(body []byte) (map[string]any, error) {
	doc, err := parseJSONObjectDoc(body, "Codex interface doc")
	if err != nil {
		return nil, err
	}
	if err := ValidateInterfaceDoc(doc); err != nil {
		return nil, err
	}
	return doc, nil
}

func ParseAppManifestDoc(body []byte) (map[string]any, error) {
	return parseJSONObjectDoc(body, "Codex app manifest")
}
