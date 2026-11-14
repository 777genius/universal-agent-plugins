package source

import "strings"

var firstPartySourceAliases = map[string]string{
	"atlassian":                "github:777genius/universal-plugins-for-ai-agents//plugins/atlassian",
	"cloudflare":               "github:777genius/universal-plugins-for-ai-agents//plugins/cloudflare",
	"cloudflare-bindings":      "github:777genius/universal-plugins-for-ai-agents//plugins/cloudflare-bindings",
	"cloudflare-docs":          "github:777genius/universal-plugins-for-ai-agents//plugins/cloudflare-docs",
	"cloudflare-observability": "github:777genius/universal-plugins-for-ai-agents//plugins/cloudflare-observability",
	"cloudflare-radar":         "github:777genius/universal-plugins-for-ai-agents//plugins/cloudflare-radar",
	"context7":                 "github:777genius/universal-plugins-for-ai-agents//plugins/context7",
	"docker-hub":               "github:777genius/universal-plugins-for-ai-agents//plugins/docker-hub",
	"firebase":                 "github:777genius/universal-plugins-for-ai-agents//plugins/firebase",
	"github":                   "github:777genius/universal-plugins-for-ai-agents//plugins/github",
	"gitlab":                   "github:777genius/universal-plugins-for-ai-agents//plugins/gitlab",
	"greptile":                 "github:777genius/universal-plugins-for-ai-agents//plugins/greptile",
	"heroku":                   "github:777genius/universal-plugins-for-ai-agents//plugins/heroku",
	"hubspot-crm":              "github:777genius/universal-plugins-for-ai-agents//plugins/hubspot-crm",
	"hubspot-developer":        "github:777genius/universal-plugins-for-ai-agents//plugins/hubspot-developer",
	"linear":                   "github:777genius/universal-plugins-for-ai-agents//plugins/linear",
	"neon":                     "github:777genius/universal-plugins-for-ai-agents//plugins/neon",
	"notion":                   "github:777genius/universal-plugins-for-ai-agents//plugins/notion",
	"sentry":                   "github:777genius/universal-plugins-for-ai-agents//plugins/sentry",
	"slack":                    "github:777genius/universal-plugins-for-ai-agents//plugins/slack",
	"stripe":                   "github:777genius/universal-plugins-for-ai-agents//plugins/stripe",
	"supabase":                 "github:777genius/universal-plugins-for-ai-agents//plugins/supabase",
	"vercel":                   "github:777genius/universal-plugins-for-ai-agents//plugins/vercel",
}

func resolveFirstPartySourceAlias(raw string) (string, bool) {
	alias := strings.ToLower(strings.TrimSpace(raw))
	resolved, ok := firstPartySourceAliases[alias]
	return resolved, ok
}
