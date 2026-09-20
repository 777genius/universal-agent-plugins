package app

import (
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/cli/internal/publicationmodel"
)

func TestRequireRemovePublicationChannelUsesLegacyPathMessage(t *testing.T) {
	t.Parallel()
	_, err := requireRemovePublicationChannel(publicationContext{target: "claude"}, publicationmodel.Model{})
	if err == nil || !strings.Contains(err.Error(), "publish/...") {
		t.Fatalf("error = %v", err)
	}
}
