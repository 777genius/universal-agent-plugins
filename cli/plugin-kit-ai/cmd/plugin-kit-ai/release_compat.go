package main

import (
	"errors"
	"strconv"
	"strings"

	"github.com/777genius/plugin-kit-ai/cli/internal/authoring/commands"
	"github.com/777genius/plugin-kit-ai/cli/internal/authoringcli"
	"github.com/spf13/cobra"
)

// This finite inventory owns retirement only. ! denotes a boolean and / a
// shorthand. All successful parsing and services remain in the shared engine.
type v1Disposition struct {
	path, flags string
	retained    bool
}

var v1Dispositions = []v1Disposition{
	{"init", "template platform runtime typescript! runtime-package! runtime-package-version output/o force!/f extras! claude-extended-hooks!", true},
	{"validate", "platform strict! format", true},
	{"inspect", "target format authoring!", true},
	{"compat", "target format from include-user-scope!", true},
	{"doctor", "", true},
	{"test", "platform event all! fixture golden-dir update-golden! format", true},
	{"capabilities", "platform format mode", true},
	{"skills", "", true},
	{"skills init", "output/o description template command force!/f", true},
	{"skills validate", "", true},
	{"skills generate", "target", false},
	{"skills install", "global!/g agent/a skill/s list!/l yes!/y copy! all! full-depth! skills-cli-version", false},
	{"skills add", "global!/g agent/a skill/s list!/l yes!/y copy! all! full-depth! skills-cli-version", false},
	{"skills list", "global!/g agent/a json! skills-cli-version", false},
	{"skills ls", "global!/g agent/a json! skills-cli-version", false},
	{"skills update", "global!/g project!/p yes!/y skills-cli-version", false},
	{"skills upgrade", "global!/g project!/p yes!/y skills-cli-version", false},
	{"skills remove", "global!/g agent/a skill/s yes!/y all! skills-cli-version", false},
	{"skills rm", "global!/g agent/a skill/s yes!/y all! skills-cli-version", false},
	{"bootstrap", "", false},
	{"dev", "platform event all! fixture golden-dir once! interval", false},
	{"generate", "target check!", false},
	{"normalize", "force!/f", false},
	{"import", "source from force!/f include-user-scope!", false},
	{"export", "platform output", false},
	{"bundle", "", false},
	{"bundle fetch", "url dest sha256 asset-name platform runtime tag latest! github-token github-api-base force!/f", false},
	{"bundle install", "dest force!/f", false},
	{"bundle publish", "platform repo tag draft! github-token github-api-base force!/f", false},
	{"publish", "channel all! dest package-root dry-run! format", false},
	{"publication", "target format", false},
	{"publication doctor", "target format dest package-root", false},
	{"publication materialize", "target dest package-root dry-run!", false},
	{"publication remove", "target dest package-root dry-run!", false},
	{"install", "tag latest! dir force!/f pre! output-name github-token goos goarch github-api-base", false},
	{"integrations", "", false},
	{"integrations add", "target scope auto-update! adopt-new-targets pre! dry-run!", false},
	{"add", "target scope auto-update! adopt-new-targets pre! dry-run!", false},
	{"integrations update", "dry-run! all!", false},
	{"update", "dry-run! all!", false},
	{"integrations remove", "dry-run!", false},
	{"remove", "dry-run!", false},
	{"integrations repair", "dry-run! target", false},
	{"repair", "dry-run! target", false},
	{"integrations list", "", false},
	{"integrations doctor", "", false},
	{"integrations sync", "dry-run!", false},
	{"integrations enable", "dry-run! target", false},
	{"integrations disable", "dry-run! target", false},
	{"version", "", true},
	{"__docs", "", false},
	{"__docs export-cli", "out-dir manifest-path", false},
	{"__docs export-support", "events-path targets-path capabilities-path", false},
}

func flagParts(spec string) (name, short string, boolean bool) {
	name, short, _ = strings.Cut(spec, "/")
	boolean = strings.HasSuffix(name, "!")
	name = strings.TrimSuffix(name, "!")
	return
}
func newReleaseRoot(factories ...authoringcli.Factory) (*cobra.Command, error) {
	root, err := authoringcli.NewReleasePluginKitRoot(factories...)
	if err != nil {
		return nil, err
	}
	for _, row := range v1Dispositions {
		c := root
		for _, part := range strings.Fields(row.path) {
			var next *cobra.Command
			for _, child := range c.Commands() {
				if child.Name() == part {
					next = child
					break
				}
			}
			if next == nil {
				next = &cobra.Command{Use: part, Hidden: true, Annotations: map[string]string{authoringcli.RejectionKey: row.path},
					RunE: func(*cobra.Command, []string) error { return errors.New("v1 operation unavailable") }}
				c.AddCommand(next)
			}
			c = next
		}
		for _, spec := range strings.Fields(row.flags) {
			name, short, boolean := flagParts(spec)
			if c.Flags().Lookup(name) != nil || root.PersistentFlags().Lookup(name) != nil {
				continue
			}
			if boolean {
				c.Flags().BoolP(name, short, false, "")
			} else {
				c.Flags().StringP(name, short, "", "")
			}
			_ = c.Flags().MarkHidden(name)
		}
	}
	return root, nil
}

func legacyGuidance(verb string) string {
	return "This v1 operation is unavailable in v2. Use plugin-kit-ai 1.2.4 with `plugin-kit-ai " + verb + "` for the legacy workflow. Project migration is unavailable in v2."
}

const absentDestination = "Use positional destination: plugin-kit-ai init <absent-destination> --name <name> --template <implemented-template> --description <description>."

func rejectV1(in commands.Invocation) error {
	verb := strings.TrimPrefix(in.Command.CommandPath(), "plugin-kit-ai ")
	has := func(name string) bool { return len(in.Values[name]) > 0 }
	last := func(name string) string {
		v := in.Values[name]
		if len(v) > 0 {
			return v[len(v)-1]
		}
		return ""
	}
	reject := func(v string, extra string) error { return errors.New(legacyGuidance(v) + " " + extra) }
	// These booleans select output or replacement intent. Validate every
	// occurrence: pflag rejects an earlier invalid value even if a later one is
	// valid. Never turn malformed input into an all/plan/JSON choice.
	for _, name := range []string{"json", "all", "dry-run"} {
		for _, value := range in.Values[name] {
			if _, err := strconv.ParseBool(value); err != nil {
				return reject(verb, "Invalid boolean flag value; use true or false.")
			}
		}
	}
	if in.Command.Annotations[authoringcli.RejectionKey] != "" {
		extra := ""
		switch verb {
		case "install":
			extra = "The retired operation is a third-party binary downloader."
		case "integrations":
			extra = "Use agentplugins --help for manager commands."
		case "integrations list", "integrations doctor":
			extra = "Manager state/health: agentplugins " + strings.TrimPrefix(verb, "integrations ") + "; report schemas and legacy-state policy differ."
		case "add", "integrations add", "update", "integrations update", "remove", "integrations remove", "repair", "integrations repair":
			supported := !has("auto-update") && !has("adopt-new-targets") && !has("pre") && (!has("scope") || last("scope") == "user")
			for _, value := range in.Values["target"] {
				for _, target := range strings.Split(value, ",") {
					switch target {
					case "claude", "codex", "gemini", "opencode", "cursor":
					default:
						supported = false
					}
				}
			}
			if supported {
				job := strings.TrimPrefix(verb, "integrations ")
				switch job {
				case "add":
					extra = "For a supported standard/local/exact source, use agentplugins add <name-or-source> --target <clients> --scope user."
				case "update":
					if in.Bool("all") {
						extra = "Use agentplugins update --all."
					} else {
						extra = "With an explicit name, use agentplugins update <name-or-installation-id>; omitted name does not imply --all."
					}
				case "remove":
					extra = "Use agentplugins remove <name-or-installation-id>."
				case "repair":
					extra = "For a supported selected manager binding, use agentplugins repair <name-or-installation-id> --target <client>."
				}
				if in.Bool("dry-run") || !has("dry-run") && strings.HasPrefix(verb, "integrations ") {
					extra += " Preserve plan intent by adding --dry-run."
				}
			}
		}
		if strings.HasPrefix(verb, "__docs") {
			return errors.New("Internal v1 documentation tooling is unavailable in this private release tree.")
		}
		return reject(verb, extra)
	}
	for _, row := range v1Dispositions {
		if row.path != verb || !row.retained {
			continue
		}
		for _, spec := range strings.Fields(row.flags) {
			name, _, _ := flagParts(spec)
			if !has(name) {
				continue
			}
			switch name {
			case "format":
				if last(name) == "text" || last(name) == "table" {
					return errors.New("Use --format human instead; --format json is retained.")
				}
			case "description": // skills init retains exactly this meaning.
			case "template":
				if verb == "init" {
					switch last(name) {
					case "skill", "mcp-remote", "mcp-stdio", "hybrid":
						continue
					}
				}
				return reject(verb, "")
			case "runtime":
				if last(name) == "node" && (last("template") == "mcp-stdio" || last("template") == "hybrid" && last("mcp-template") == "mcp-stdio") {
					continue
				}
				return reject("init --runtime", "")
			case "target":
				for _, value := range in.Values[name] {
					for _, target := range strings.Split(value, ",") {
						switch target {
						case "all":
							return errors.New("Select explicit comma-separated clients, for example --target claude,codex.")
						case "codex-package", "codex-runtime", "cursor-workspace":
							return reject(verb, "Standard static compatibility can separately use --target codex or --target cursor; legacy runtime/workspace semantics are unavailable.")
						}
					}
				}
			case "output":
				if verb == "init" {
					return errors.New(absentDestination)
				}
				return errors.New("Use positional package root: plugin-kit-ai skills init <name> [package-path] --description <description>.")
			case "force":
				if verb == "init" {
					return reject("init --force", "init requires an absent destination; overwriting is unavailable. "+absentDestination)
				}
				return reject(verb, "Use plugin-kit-ai skills init <name> [package-path] --description <description> only for a new standard Skill.")
			case "authoring":
				return errors.New("Standard inspection is inherent; omit --authoring.")
			default:
				if verb == "validate" {
					return reject(verb+" --"+name, "")
				}
				return reject(verb, "")
			}
		}
	}
	return nil
}
