package management

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestLocalSkillNullableFieldsMustBeExplicit(t *testing.T) {
	for _, value := range []any{
		map[string]any{"relativePath": "SKILL.md", "expectedSHA256": strings.Repeat("a", 64)},
		map[string]any{"relativePath": "SKILL.md", "contentBase64": "eA=="},
		map[string]any{"relativePath": "SKILL.md", "expectedSHA256": nil, "contentBase64": "eA==", "extra": "not allowed"},
		map[string]any{"relativePath": "SKILL.md", "expectedSHA256": nil, "contentBase64": false},
		nil,
	} {
		body, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		var file LocalSkillFile
		if err := json.Unmarshal(body, &file); err == nil {
			t.Fatalf("malformed nullable file accepted: %s", body)
		}
	}
	var create, remove LocalSkillFile
	if err := json.Unmarshal([]byte(`{"relativePath":"SKILL.md","expectedSHA256":null,"contentBase64":"eA=="}`), &create); err != nil {
		t.Fatal(err)
	}
	if create.ExpectedSHA256 != nil || create.ContentBase64 == nil || *create.ContentBase64 != "eA==" {
		t.Fatal("explicit absence lost", create)
	}
	if err := json.Unmarshal([]byte(`{"relativePath":"SKILL.md","expectedSHA256":"digest","contentBase64":null}`), &remove); err != nil {
		t.Fatal(err)
	}
	if remove.ExpectedSHA256 == nil || *remove.ExpectedSHA256 != "digest" || remove.ContentBase64 != nil {
		t.Fatal("explicit deletion lost", remove)
	}
}
