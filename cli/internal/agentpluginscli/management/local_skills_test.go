package management

import (
	"strings"
	"sync/atomic"
	"testing"
)

func TestLocalSkillNullableFieldsMustBeExplicit(t *testing.T) {
	var prepared atomic.Int32
	server := newTestServer(t, "", "capture", &testBackend{makePlan: func(ChangeRequest, string) Prepared {
		prepared.Add(1)
		return &testPrepared{}
	}})
	request := func(file any, status int) {
		t.Helper()
		body := map[string]any{"action": "upsert", "rootId": "local", "skillName": "draft", "files": []any{file}}
		requestJSON(t, server, "POST", "/api/v1/skills/local/plans", body, status, nil)
	}
	for _, file := range []any{
		map[string]any{"relativePath": "SKILL.md", "expectedSHA256": strings.Repeat("a", 64)},
		map[string]any{"relativePath": "SKILL.md", "contentBase64": "eA=="},
		map[string]any{"relativePath": "SKILL.md", "expectedSHA256": nil, "contentBase64": "eA==", "extra": "not allowed"},
		map[string]any{"relativePath": "SKILL.md", "expectedSHA256": nil, "contentBase64": false},
		nil,
	} {
		request(file, 400)
	}
	if prepared.Load() != 0 {
		t.Fatal("malformed nullable input reached preparation")
	}
	request(map[string]any{"relativePath": "SKILL.md", "expectedSHA256": nil, "contentBase64": "eA=="}, 200)
	request(map[string]any{"relativePath": "SKILL.md", "expectedSHA256": strings.Repeat("a", 64), "contentBase64": nil}, 200)
	if prepared.Load() != 2 {
		t.Fatal("explicit absence or deletion was not preserved")
	}
}
