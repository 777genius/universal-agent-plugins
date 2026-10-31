// Package all assembles the registry of every client adapter shipped in this
// repository. It is the single place that names them all, so it is also the
// only package that links every adapter into a binary: the composition root and
// tests import it, generic packages never do.
package all

import (
	"fmt"
	"sync"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/chatgpt"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/claude"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/cline"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/codex"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/copilot"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/cursor"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/gemini"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/grok"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/kimi"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/kiro"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/opencode"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/pi"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/qoder"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/vscode"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/windsurf"
)

// Default reports the registry of every in-tree client adapter. The registry is
// immutable once built, so one instance is shared.
func Default() *clients.Registry { return defaultRegistry() }

// Project selects explicit workspace adapters while retaining every other
// client's own project admission. SDK hosts use it with a project-scoped engine.
// Default retains VS Code's user/Copilot backend and refuses its project scope.
func Project() *clients.Registry { return projectRegistry() }

// A failure here is a duplicate or an unknown client id, which is a mistake in
// the list below rather than a runtime condition, so it panics instead of
// handing back an error every caller would have to translate into the same
// crash. It is built on first use rather than in init, so importing the
// composition root of one command does not pay for it.
var defaultRegistry = sync.OnceValue(func() *clients.Registry { return buildRegistry(false) })
var projectRegistry = sync.OnceValue(func() *clients.Registry { return buildRegistry(true) })

func buildRegistry(project bool) *clients.Registry {
	var workspace clients.Adapter = vscode.New()
	if project {
		workspace = vscode.NewProject()
	}
	registry, err := clients.NewRegistry(
		pi.New(),
		codex.New(),
		chatgpt.New(),
		cursor.New(),
		copilot.New(),
		workspace,
		kiro.New(),
		claude.New(),
		gemini.New(),
		opencode.New(),
		cline.New(),
		windsurf.New(),
		grok.New(),
		kimi.New(),
		qoder.New(),
	)
	if err != nil {
		panic(fmt.Sprintf("build the default client registry: %v", err))
	}
	return registry
}
