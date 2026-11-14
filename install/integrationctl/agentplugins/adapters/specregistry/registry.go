package specregistry

import (
	"bytes"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"encoding/json"
	"fmt"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/dlclark/regexp2"
	jsonschema "github.com/santhosh-tekuri/jsonschema/v6"
)

const (
	pluginSchemaPath   = "schemas/1.0.0/plugin.schema.json"
	mcpSchemaPath      = "schemas/1.0.0/mcp.schema.json"
	pluginSchemaDigest = "sha256:0a4aad95ce337878ad38802ebf0daa3fde76abe3f65400c86bcbb1ec0b3ab883"
	mcpSchemaDigest    = "sha256:6539175bfcdf43085855183e86da40ea94b166547a72b47ae9a0a390516d3acb"
)

//go:embed schemas/1.0.0/*.json
var schemas embed.FS

type Registry struct {
	compiled map[string]*jsonschema.Schema
	digests  map[string]string
}

func New() (*Registry, error) {
	compiler := jsonschema.NewCompiler()
	compiler.UseLoader(rejectExternalLoader{})
	compiler.UseRegexpEngine(compileECMAScriptRegexp)
	definitions := []struct {
		uri            string
		path           string
		expectedDigest string
	}{
		{domain.PluginSchemaV1, pluginSchemaPath, pluginSchemaDigest},
		{domain.MCPSchemaV1, mcpSchemaPath, mcpSchemaDigest},
	}
	registry := &Registry{
		compiled: make(map[string]*jsonschema.Schema, len(definitions)),
		digests:  make(map[string]string, len(definitions)),
	}
	for _, definition := range definitions {
		body, err := schemas.ReadFile(definition.path)
		if err != nil {
			return nil, fmt.Errorf("read embedded schema %s: %w", definition.uri, err)
		}
		sum := sha256.Sum256(body)
		digest := "sha256:" + hex.EncodeToString(sum[:])
		if digest != definition.expectedDigest {
			return nil, fmt.Errorf("embedded schema %s digest %s does not match pinned %s", definition.uri, digest, definition.expectedDigest)
		}
		var document any
		decoder := json.NewDecoder(bytes.NewReader(body))
		decoder.UseNumber()
		if err := decoder.Decode(&document); err != nil {
			return nil, fmt.Errorf("decode embedded schema %s: %w", definition.uri, err)
		}
		if err := compiler.AddResource(definition.uri, document); err != nil {
			return nil, fmt.Errorf("register embedded schema %s: %w", definition.uri, err)
		}
		registry.digests[definition.uri] = digest
	}
	for _, definition := range definitions {
		compiled, err := compiler.Compile(definition.uri)
		if err != nil {
			return nil, fmt.Errorf("compile embedded schema %s: %w", definition.uri, err)
		}
		registry.compiled[definition.uri] = compiled
	}
	return registry, nil
}

func (registry *Registry) Supports(schemaURI string) bool {
	if registry == nil {
		return false
	}
	_, ok := registry.compiled[schemaURI]
	return ok
}

func (registry *Registry) Validate(schemaURI string, value any) error {
	if registry == nil {
		return fmt.Errorf("schema registry is not initialized")
	}
	schema, ok := registry.compiled[schemaURI]
	if !ok {
		return fmt.Errorf("unsupported Agent Plugins schema %q", schemaURI)
	}
	return schema.Validate(value)
}

func (registry *Registry) Digest(schemaURI string) (string, bool) {
	if registry == nil {
		return "", false
	}
	digest, ok := registry.digests[schemaURI]
	return digest, ok
}

type rejectExternalLoader struct{}

func (rejectExternalLoader) Load(url string) (any, error) {
	return nil, fmt.Errorf("external schema loading is disabled: %s", url)
}

type ecmaScriptRegexp regexp2.Regexp

func (regexp *ecmaScriptRegexp) MatchString(value string) bool {
	matched, err := (*regexp2.Regexp)(regexp).MatchString(value)
	return err == nil && matched
}

func (regexp *ecmaScriptRegexp) String() string {
	return (*regexp2.Regexp)(regexp).String()
}

func compileECMAScriptRegexp(expression string) (jsonschema.Regexp, error) {
	compiled, err := regexp2.Compile(expression, regexp2.ECMAScript)
	if err != nil {
		return nil, err
	}
	return (*ecmaScriptRegexp)(compiled), nil
}
