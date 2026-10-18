package agentpluginscli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/pelletier/go-toml/v2"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/environment"
)

type rawDependency struct {
	Version      string `toml:"-"`
	Git          string `toml:"git"`
	Path         string `toml:"path"`
	Ref          string `toml:"ref"`
	Commit       string `toml:"commit"`
	Registry     string `toml:"registry"`
	Distribution string `toml:"distribution"`
	Release      any    `toml:"release"`
}

type dependencyManifest struct {
	SchemaVersion int                 `toml:"schema_version"`
	Dependencies  map[string]any      `toml:"dependencies"`
	Targets       map[string][]string `toml:"targets"`
}

func (app App) dependencyPaths() (manifest, lock string) {
	root := strings.TrimSpace(app.DependenciesRoot)
	if root == "" {
		root = filepath.Dir(app.ManagedRoot)
	}
	return filepath.Join(root, "uap.toml"), filepath.Join(root, "uap.lock")
}

func readDependencyManifest(path string) (environment.Requirements, error) {
	body, err := os.ReadFile(path)
	if err != nil {
		return environment.Requirements{}, err
	}
	return readDependencyManifestBytes(path, body)
}

func readDependencyManifestBytes(path string, body []byte) (environment.Requirements, error) {
	var generic map[string]any
	if err := toml.Unmarshal(body, &generic); err != nil {
		return environment.Requirements{}, manifestError(path, body, err)
	}
	if err := rejectManifestUnknown(path, body, generic); err != nil {
		return environment.Requirements{}, err
	}
	var raw dependencyManifest
	if err := toml.Unmarshal(body, &raw); err != nil {
		return environment.Requirements{}, manifestError(path, body, err)
	}
	requirements := environment.Requirements{SchemaVersion: raw.SchemaVersion, Dependencies: map[string]environment.Requirement{}, Targets: map[string][]string{}}
	for alias, item := range raw.Dependencies {
		rawItem, err := decodeRawDependency(item)
		if err != nil {
			return environment.Requirements{}, manifestError(path, body, err, "dependencies", alias)
		}
		requirement, err := requirementFromRaw(alias, rawItem)
		if err != nil {
			return environment.Requirements{}, manifestError(path, body, fmt.Errorf("dependencies.%s: %w", alias, err), "dependencies", alias)
		}
		if err := requirement.Validate(); err != nil {
			return environment.Requirements{}, manifestError(path, body, err, "dependencies", alias)
		}
		requirements.Dependencies[alias] = requirement
	}
	for target, aliases := range raw.Targets {
		requirements.Targets[target] = append([]string(nil), aliases...)
	}
	if err := requirements.Validate(); err != nil {
		return environment.Requirements{}, manifestError(path, body, err, requirementErrorLocation(requirements, err)...)
	}
	return requirements, nil
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
	if gitSource == registrySource {
		return environment.Requirement{}, errors.New("dependency must contain exactly one git or registry source")
	}
	if gitSource {
		if raw.Registry != "" || raw.Distribution != "" || raw.Release != nil {
			return environment.Requirement{}, errors.New("git source cannot contain registry fields")
		}
		return environment.Requirement{Alias: alias, Git: &environment.GitSource{Repository: strings.ToLower(strings.TrimSuffix(strings.TrimPrefix(raw.Git, "https://github.com/"), ".git")), Path: raw.Path, Selector: selector}}, nil
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
	for key := range generic {
		if key != "schema_version" && key != "dependencies" && key != "targets" {
			return manifestError(path, body, fmt.Errorf("unknown top-level field %q", key), key)
		}
	}
	dependencies, ok := generic["dependencies"].(map[string]any)
	if ok {
		for alias, value := range dependencies {
			if expression, ok := value.(string); ok {
				if expression == "" {
					return manifestError(path, body, errors.New("dependency version must not be empty"), "dependencies", alias)
				}
				continue
			}
			fields, ok := value.(map[string]any)
			if !ok {
				return manifestError(path, body, fmt.Errorf("dependencies.%s must be a version string or table", alias), "dependencies", alias)
			}
			for key := range fields {
				switch key {
				case "git", "path", "ref", "commit", "registry", "distribution", "release":
				default:
					return manifestError(path, body, fmt.Errorf("dependencies.%s contains unknown field %q", alias, key), "dependencies", alias, key)
				}
			}
		}
	}
	return nil
}

func writeInitialDependencyManifest(path string) error {
	body := initialDependencyManifest()
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	if _, err = file.Write(body); err == nil {
		err = file.Sync()
	}
	closeErr := file.Close()
	if err != nil {
		return err
	}
	return closeErr
}

func initialDependencyManifest() []byte {
	return []byte(`# Agent Plugins dependency environment
schema_version = 1

[dependencies]

[targets]
`)
}
