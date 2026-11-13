package agentpluginscli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	jsonschema "github.com/santhosh-tekuri/jsonschema/v6"
)

// Run only in the dedicated CI lane that downloads immutable official schemas.
// Unit tests remain offline and require no new production dependencies.
func TestDependencySBOMAgainstOfficialSchemas(t *testing.T) {
	root := os.Getenv("UAP_SBOM_SCHEMA_DIR")
	if root == "" {
		t.Skip("official schema conformance is checked in the dedicated SBOM CI job")
	}
	type schemaFile struct {
		name, uri string
	}
	files := []schemaFile{
		{"bom-1.5.schema.json", "http://cyclonedx.org/schema/bom-1.5.schema.json"},
		{"spdx.schema.json", "http://cyclonedx.org/schema/spdx.schema.json"},
		{"jsf-0.82.schema.json", "http://cyclonedx.org/schema/jsf-0.82.schema.json"},
		{"spdx-2.3.schema.json", "http://spdx.org/rdf/terms/2.3"},
	}
	compiler := jsonschema.NewCompiler()
	compiler.UseLoader(denySBOMSchemaNetwork{})
	for _, file := range files {
		body, err := os.ReadFile(filepath.Join(root, file.name))
		if err != nil {
			t.Fatalf("read pinned schema %s: %v", file.name, err)
		}
		var document any
		if err := json.Unmarshal(body, &document); err != nil {
			t.Fatalf("parse pinned schema %s: %v", file.name, err)
		}
		if err := compiler.AddResource(file.uri, document); err != nil {
			t.Fatalf("register pinned schema %s: %v", file.name, err)
		}
	}
	for _, tc := range []struct {
		kind, uri, mandatory string
	}{
		{"cyclonedx", files[0].uri, "bomFormat"},
		{"spdx", files[3].uri, "SPDXID"},
	} {
		t.Run(tc.kind, func(t *testing.T) {
			schema, err := compiler.Compile(tc.uri)
			if err != nil {
				t.Fatalf("compile pinned official schema: %v", err)
			}
			encoded, err := marshalDependencySBOM(testSBOMLock(), tc.kind)
			if err != nil {
				t.Fatal(err)
			}
			decoder := json.NewDecoder(bytes.NewReader(encoded))
			decoder.UseNumber()
			var document map[string]any
			if err := decoder.Decode(&document); err != nil {
				t.Fatal(err)
			}
			if err := schema.Validate(document); err != nil {
				t.Fatalf("%s exported invalid official-schema document: %v", tc.kind, err)
			}
			delete(document, tc.mandatory)
			if err := schema.Validate(document); err == nil {
				t.Fatal("schema validation did not reject a missing mandatory root field")
			}
		})
	}
}

type denySBOMSchemaNetwork struct{}

func (denySBOMSchemaNetwork) Load(uri string) (any, error) {
	return nil, fmt.Errorf("unexpected network schema dependency %q", uri)
}
