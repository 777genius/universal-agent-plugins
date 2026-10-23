// Package dependencymanifest decodes the UAP manager's dependency sidecar.
// It performs no filesystem access, source lookup or target discovery.
package dependencymanifest

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/pelletier/go-toml/v2"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/environment"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/sourcespec"
)

type rawDependency struct {
	Archive      string `toml:"archive"`
	SHA256       string `toml:"sha256"`
	Version      string `toml:"-"`
	Git          string `toml:"git"`
	Path         string `toml:"path"`
	Ref          string `toml:"ref"`
	Commit       string `toml:"commit"`
	Registry     string `toml:"registry"`
	Distribution string `toml:"distribution"`
	Release      any    `toml:"release"`
}

// Document contains decoded declarations and source locations. Its views return
// independent values so consumers cannot change the captured document.
type Document struct {
	source       string
	body         []byte
	requirements environment.Requirements
}

// Decode validates the common grammar, source expressions and schema version.
// Root target authority and package-only source restrictions belong to the views.
func Decode(body []byte, source string) (Document, error) {
	var raw map[string]any
	if err := toml.Unmarshal(body, &raw); err != nil {
		return Document{}, manifestError(source, body, err)
	}
	if err := rejectManifestUnknown(source, body, raw); err != nil {
		return Document{}, err
	}
	schema, ok := raw["schema_version"].(int64)
	if !ok || schema != environment.ManifestSchemaVersion {
		return Document{}, manifestError(source, body, errors.New("schema_version must be integer 1"), "schema_version")
	}
	dependencies, err := decodeDependencies(raw, source, body)
	if err != nil {
		return Document{}, err
	}
	targets, err := decodeTargets(raw, source, body)
	if err != nil {
		return Document{}, err
	}
	reqs := environment.Requirements{SchemaVersion: int(schema), Dependencies: dependencies, Targets: targets}
	return Document{source: source, body: append([]byte(nil), body...), requirements: reqs}, nil
}

func decodeDependencies(raw map[string]any, source string, body []byte) (map[string]environment.Requirement, error) {
	dependencies := map[string]environment.Requirement{}
	value, exists := raw["dependencies"]
	if !exists {
		return dependencies, nil
	}
	entries, ok := value.(map[string]any)
	if !ok {
		return nil, manifestError(source, body, errors.New("dependencies must be a table"), "dependencies")
	}
	sources := map[string]string{}
	for _, alias := range sortedKeys(entries) {
		item, err := decodeRawDependency(entries[alias])
		if err != nil {
			return nil, manifestError(source, body, err, "dependencies", alias)
		}
		req, err := requirementFromRaw(alias, item)
		if err == nil {
			err = req.Validate()
		}
		if err != nil {
			return nil, manifestError(source, body, fmt.Errorf("dependencies.%s: %w", alias, err), "dependencies", alias)
		}
		key, err := req.SourceKey()
		if err != nil {
			return nil, manifestError(source, body, err, "dependencies", alias)
		}
		if previous, exists := sources[key]; exists {
			return nil, manifestError(source, body, fmt.Errorf("dependencies %q and %q use the same source %q", previous, alias, key), "dependencies", alias)
		}
		sources[key] = alias
		dependencies[alias] = req
	}
	return dependencies, nil
}

func decodeTargets(raw map[string]any, source string, body []byte) (map[string][]string, error) {
	targets := map[string][]string{}
	value, exists := raw["targets"]
	if !exists {
		return targets, nil
	}
	entries, ok := value.(map[string]any)
	if !ok {
		return nil, manifestError(source, body, errors.New("targets must be a table"), "targets")
	}
	for _, target := range sortedKeys(entries) {
		values, ok := entries[target].([]any)
		if !ok {
			return nil, manifestError(source, body, errors.New("target must be an array of strings"), "targets", target)
		}
		aliases := make([]string, 0, len(values))
		for _, value := range values {
			alias, ok := value.(string)
			if !ok {
				return nil, manifestError(source, body, errors.New("target must be an array of strings"), "targets", target)
			}
			aliases = append(aliases, alias)
		}
		targets[target] = aliases
	}
	return targets, nil
}

// Root checks target references and requires every declaration to be assigned.
// The caller still checks target IDs against its own supported-target authority.
func (doc Document) Root() (environment.Requirements, error) {
	reqs := doc.clone()
	if err := reqs.Validate(); err != nil {
		return environment.Requirements{}, manifestError(doc.source, doc.body, err, requirementErrorLocation(reqs, err)...)
	}
	return reqs, nil
}

// Package exports only requirements; structurally valid author targets have no
// authority over the consumer, including their target IDs and local references.
func (doc Document) Package() (map[string]environment.Requirement, error) {
	if doc.requirements.SchemaVersion != environment.ManifestSchemaVersion {
		return nil, manifestError(doc.source, doc.body, errors.New("schema_version must be integer 1"), "schema_version")
	}
	reqs := doc.clone()
	for _, alias := range sortedKeys(reqs.Dependencies) {
		req := reqs.Dependencies[alias]
		if req.Registry != nil && strings.Count(req.Registry.Distribution, "/") != 1 {
			return nil, manifestError(doc.source, doc.body, fmt.Errorf("dependencies.%s: package registry distribution must be canonical owner/name", alias), "dependencies", alias)
		}
	}
	return reqs.Dependencies, nil
}

func (doc Document) clone() environment.Requirements {
	reqs := environment.Requirements{SchemaVersion: doc.requirements.SchemaVersion, Dependencies: map[string]environment.Requirement{}, Targets: map[string][]string{}}
	for key, value := range doc.requirements.Dependencies {
		if value.Git != nil {
			source := *value.Git
			value.Git = &source
		}
		if value.Archive != nil {
			source := *value.Archive
			value.Archive = &source
		}
		if value.Registry != nil {
			source := *value.Registry
			value.Registry = &source
		}
		reqs.Dependencies[key] = value
	}
	for key, value := range doc.requirements.Targets {
		reqs.Targets[key] = append([]string(nil), value...)
	}
	return reqs
}

func decodeRawDependency(value any) (rawDependency, error) {
	if version, ok := value.(string); ok {
		return rawDependency{Version: version}, nil
	}
	body, err := toml.Marshal(map[string]any{"dependency": value})
	if err != nil {
		return rawDependency{}, err
	}
	var decoded struct {
		Dependency rawDependency `toml:"dependency"`
	}
	if err := toml.Unmarshal(body, &decoded); err != nil {
		return rawDependency{}, err
	}
	return decoded.Dependency, nil
}

func requirementFromRaw(alias string, raw rawDependency) (environment.Requirement, error) {
	if raw.Version != "" {
		selector, err := rawSelector(raw)
		return environment.Requirement{Alias: alias, Registry: &environment.RegistrySource{Distribution: alias, Selector: selector}}, err
	}
	selector, err := rawSelector(raw)
	if err != nil {
		return environment.Requirement{}, err
	}
	gitSource, registrySource := raw.Git != "", raw.Registry != "" || raw.Distribution != "" || raw.Release != nil
	count := 0
	for _, yes := range []bool{gitSource, registrySource, raw.Archive != ""} {
		if yes {
			count++
		}
	}
	if count != 1 {
		return environment.Requirement{}, errors.New("dependency must contain exactly one git, archive or registry source")
	}
	if raw.Archive != "" {
		if raw.Ref != "" || raw.Commit != "" {
			return environment.Requirement{}, errors.New("archive source cannot contain Git selectors")
		}
		endpoint, err := sourcespec.NormalizeArchiveURL(raw.Archive)
		if err != nil {
			return environment.Requirement{}, err
		}
		return environment.Requirement{Alias: alias, Archive: &environment.ArchiveSource{URL: endpoint, Path: raw.Path, SHA256: raw.SHA256}}, nil
	}
	if raw.SHA256 != "" {
		return environment.Requirement{}, errors.New("sha256 is only valid for archive sources")
	}
	if gitSource {
		if raw.Registry != "" || raw.Distribution != "" || raw.Release != nil {
			return environment.Requirement{}, errors.New("git source cannot contain registry fields")
		}
		endpoint := raw.Git
		if !strings.Contains(endpoint, ":") {
			endpoint = "https://github.com/" + endpoint + ".git"
		}
		normalized, repository, err := sourcespec.NormalizeGitURL(endpoint)
		if err != nil {
			return environment.Requirement{}, err
		}
		source := &environment.GitSource{Repository: repository, Path: raw.Path, Selector: selector}
		if repository == "" {
			source.URL = normalized
		}
		return environment.Requirement{Alias: alias, Git: source}, nil
	}
	if raw.Ref != "" || raw.Commit != "" {
		return environment.Requirement{}, errors.New("registry source cannot contain git selectors")
	}
	if raw.Registry != "uap" {
		return environment.Requirement{}, errors.New("registry must be uap")
	}
	if raw.Path != "" {
		return environment.Requirement{}, errors.New("registry source cannot contain git path")
	}
	return environment.Requirement{Alias: alias, Registry: &environment.RegistrySource{Distribution: strings.TrimSpace(raw.Distribution), Selector: selector}}, nil
}

func rawSelector(raw rawDependency) (environment.Selector, error) {
	if raw.Version != "" {
		kind := environment.SelectorVersion
		if strings.HasPrefix(raw.Version, "^") {
			kind = environment.SelectorRange
		}
		return environment.Selector{Kind: kind, Value: raw.Version}, nil
	}
	if raw.Archive != "" {
		return environment.Selector{Kind: environment.SelectorArchive, Value: raw.SHA256}, nil
	}
	if raw.Git != "" {
		switch {
		case raw.Commit != "" && raw.Ref != "":
			return environment.Selector{}, errors.New("commit and ref are mutually exclusive")
		case raw.Commit != "":
			return environment.Selector{Kind: environment.SelectorCommit, Value: raw.Commit}, nil
		case raw.Ref != "":
			return environment.Selector{Kind: environment.SelectorRef, Value: raw.Ref}, nil
		default:
			return environment.Selector{Kind: environment.SelectorDefault}, nil
		}
	}
	if raw.Release == nil {
		return environment.Selector{}, errors.New("registry source requires exact release")
	}
	var value string
	switch release := raw.Release.(type) {
	case int64:
		value = strconv.FormatInt(release, 10)
	case uint64:
		value = strconv.FormatUint(release, 10)
	case string:
		value = release
	default:
		return environment.Selector{}, errors.New("release must be an integer sequence")
	}
	return environment.Selector{Kind: environment.SelectorRelease, Value: value}, nil
}

func rejectManifestUnknown(path string, body []byte, generic map[string]any) error {
	for _, key := range sortedKeys(generic) {
		if key != "schema_version" && key != "dependencies" && key != "targets" {
			return manifestError(path, body, fmt.Errorf("unknown top-level field %q", key), key)
		}
	}
	dependencies, _ := generic["dependencies"].(map[string]any)
	for _, alias := range sortedKeys(dependencies) {
		if err := validateDependencyFields(path, body, alias, dependencies[alias]); err != nil {
			return err
		}
	}
	return nil
}

func validateDependencyFields(path string, body []byte, alias string, value any) error {
	if expression, ok := value.(string); ok {
		if expression == "" {
			return manifestError(path, body, errors.New("dependency version must not be empty"), "dependencies", alias)
		}
		return nil
	}
	fields, ok := value.(map[string]any)
	if !ok {
		return manifestError(path, body, fmt.Errorf("dependencies.%s must be a version string or table", alias), "dependencies", alias)
	}
	for _, key := range sortedKeys(fields) {
		switch key {
		case "git", "path", "ref", "commit", "registry", "distribution", "archive", "sha256":
			if _, ok := fields[key].(string); !ok {
				return manifestError(path, body, fmt.Errorf("dependencies.%s.%s must be a string", alias, key), "dependencies", alias, key)
			}
		case "release":
		default:
			return manifestError(path, body, fmt.Errorf("dependencies.%s contains unknown field %q", alias, key), "dependencies", alias, key)
		}
	}
	return nil
}
