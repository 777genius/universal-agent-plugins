package providers

import (
	"context"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/conformance"
)

// decodeSkillFrontmatter preserves the complete canonical frontmatter and
// instructions used by the ordinary native Skill catalog. Unknown keys remain
// visible to the existing all-fields comparison; this is not a second parser.
func decodeSkillFrontmatter(ctx context.Context, body []byte) (map[string]any, []byte, error) {
	front, err := conformance.DecodeFrontmatter(ctx, body, conformance.DefaultLimits(), nil)
	if err != nil {
		return nil, nil, err
	}
	fields := make(map[string]struct{}, len(front.Keys))
	for _, key := range front.Keys {
		fields[key] = struct{}{}
	}
	front, err = conformance.DecodeFrontmatter(ctx, body, conformance.DefaultLimits(), fields)
	if err != nil {
		return nil, nil, err
	}
	return front.Values, front.Body, nil
}
