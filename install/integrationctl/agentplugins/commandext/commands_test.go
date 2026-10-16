package commandext_test

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/commandext"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/conformance"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

const validHeader = "---\ndescription: Say hello\ndisable-model-invocation: false\n---\n"

func file(name, body string) commandext.Entry {
	return commandext.Entry{Name: name, Kind: commandext.EntryFile, Body: []byte(body)}
}

func load(t *testing.T, plugin string, skills []string, entries ...commandext.Entry) domain.PackageEnvelope {
	t.Helper()
	envelope := domain.PackageEnvelope{Manifest: domain.PluginManifest{Name: plugin}, SkillsDirEntries: skills}
	result, err := commandext.WithCommands(context.Background(), envelope, commandext.Listing{Root: commandext.RootDirectory, Entries: entries})
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func codes(envelope domain.PackageEnvelope, item string) []string {
	var result []string
	for _, diagnostic := range envelope.Diagnostics {
		if diagnostic.Item == item && diagnostic.Boundary == domain.BoundaryCommand {
			result = append(result, diagnostic.Code)
		}
	}
	return result
}

// The format is closed and every rule has a fixed code and position, so an
// author's CI can rely on the exact diagnostics. Each case names the codes in
// the order they must come out.
func TestCommandCodesAndOrder(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name, body string
		want       []string
	}{
		{"valid", validHeader + "Hello $CLAUDE_NOTIFICATIONS_BIN\n", nil},
		{"crlf", strings.ReplaceAll(validHeader+"Hello\n", "\n", "\r\n"), nil},
		{"unknown-field", "---\ndescription: d\ndisable-model-invocation: true\nallowed-tools: Bash\n---\nbody\n", []string{"command_unknown_field"}},
		{"merge-key", "---\nbase: &b {description: d}\n<<: *b\ndisable-model-invocation: true\n---\nbody\n", []string{"command_unknown_field"}},
		{"string-flag", "---\ndescription: d\ndisable-model-invocation: \"true\"\n---\nbody\n", []string{"command_invocation_flag_invalid"}},
		{"missing-flag", "---\ndescription: d\n---\nbody\n", []string{"command_invocation_flag_invalid"}},
		{"blank-description", "---\ndescription: \"   \"\ndisable-model-invocation: true\n---\nbody\n", []string{"command_description_invalid"}},
		{"multiline-description", "---\ndescription: \"a\\nb\"\ndisable-model-invocation: true\n---\nbody\n", []string{"command_description_invalid"}},
		{"duplicate-key", "---\ndescription: d\ndescription: e\ndisable-model-invocation: true\n---\nbody\n", []string{"command_frontmatter_invalid"}},
		{"no-frontmatter", "Run with $1 please\n", []string{"command_frontmatter_invalid", "command_argument_placeholder"}},
		{"bad-yaml-keeps-body", "---\ndescription: [\n---\n!`date`\n", []string{"command_frontmatter_invalid", "command_shell_injection"}},
		{"plugin-root-once", validHeader + "${CLAUDE_PLUGIN_ROOT}/bin and $PLUGIN_DATA\n", []string{"command_plugin_variable"}},
		{"client-variable", validHeader + "${CLAUDE_SESSION_ID} ${CLAUDE_PLUGIN_ROOT_EXTRA}\n", []string{"command_client_variable"}},
		{"long-fence", validHeader + "````!\nrm -rf /\n````\n", []string{"command_shell_injection"}},
		{"list-fence", validHeader + "- ```!\nrm -rf /\n  ```\n", []string{"command_shell_injection"}},
		{"quote-fence", validHeader + "> ```!\n> date\n> ```\n", []string{"command_shell_injection"}},
		{"inline-fence", validHeader + "see ```!date``` here\n", []string{"command_shell_injection"}},
		{"cr-fence", validHeader + "text\r```!\ndate\n```\n", []string{"command_shell_injection"}},
		{"indented-tilde-fence", validHeader + " 1. ~~~!\ndate\n~~~\n", []string{"command_shell_injection"}},
		{"plain-fences", validHeader + "```bash\necho $HOME $CLAUDE_NOTIFICATIONS_BIN\n```\nDone! Run `make`!\n~~~\nok\n~~~\n", nil},
		{"nul-description", "---\ndescription: \"a\\0b\"\ndisable-model-invocation: true\n---\nbody\n", []string{"command_description_invalid"}},
		{"separator-description", "---\ndescription: \"a\\u2028b\"\ndisable-model-invocation: true\n---\nbody\n", []string{"command_description_invalid"}},
		{"control-description", "---\ndescription: \"a\\x07b\"\ndisable-model-invocation: true\n---\nbody\n", []string{"command_description_invalid"}},
		{"gemini-shell", validHeader + "!{ls}\n", []string{"command_shell_injection"}},
		{"args-template", validHeader + "{{ args }}\n", []string{"command_argument_placeholder"}},
		{"empty-body", validHeader + " \n\t\n", []string{"command_body_empty"}},
		{"nul-body", validHeader + "a\x00b\n", []string{"command_body_encoding"}},
		{"everything", "---\nfoo: 1\n---\n${CLAUDE_SESSION_ID} $ARGUMENTS !{x}\n", []string{"command_unknown_field", "command_description_invalid", "command_invocation_flag_invalid", "command_client_variable", "command_argument_placeholder", "command_shell_injection"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			envelope := load(t, "demo", nil, file(tc.name+".md", tc.body))
			if got := codes(envelope, tc.name); !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("codes = %v, want %v", got, tc.want)
			}
			_, delivered := envelope.Commands[tc.name]
			if delivered != (tc.want == nil) {
				t.Fatalf("command delivered = %v with codes %v", delivered, tc.want)
			}
		})
	}
}

func TestCommandDecodesFieldsAndBody(t *testing.T) {
	t.Parallel()
	body := "---\r\ndescription: Open settings\r\ndisable-model-invocation: true\r\n---\r\nStep one\r\n"
	command := load(t, "demo", nil, file("settings.md", body)).Commands["settings"]
	want := domain.Command{Name: "settings", Description: "Open settings", ExplicitOnly: true, Body: []byte("Step one\n"), RelativePath: commandext.RelativeDirectory + "/settings.md"}
	if !reflect.DeepEqual(command, want) {
		t.Fatalf("command = %#v, want %#v", command, want)
	}
}

func TestCommandEntriesOutsideTheFormat(t *testing.T) {
	t.Parallel()
	envelope := load(t, "demo", nil,
		commandext.Entry{Name: ".gitkeep", Kind: commandext.EntryFile},
		commandext.Entry{Name: ".hidden", Kind: commandext.EntryDirectory},
		commandext.Entry{Name: ".link.md", Kind: commandext.EntrySymlink},
		file("Upper.MD", validHeader+"x\n"),
		file("con.md", validHeader+"x\n"),
		commandext.Entry{Name: "nested", Kind: commandext.EntryDirectory},
		commandext.Entry{Name: "link.md", Kind: commandext.EntrySymlink},
		commandext.Entry{Name: "broken.md", Kind: commandext.EntryFile, Unreadable: true},
		file("ok.md", validHeader+"x\n"),
	)
	if want := []string{"Upper.MD", "broken", "con", "link", "nested"}; !reflect.DeepEqual(envelope.InvalidCommands, want) {
		t.Fatalf("invalid = %v, want %v", envelope.InvalidCommands, want)
	}
	if got := codes(envelope, "broken"); !reflect.DeepEqual(got, []string{"command_read_failed"}) {
		t.Fatalf("unreadable codes = %v", got)
	}
	for _, item := range []string{"Upper.MD", "con", "link", "nested"} {
		if got := codes(envelope, item); !reflect.DeepEqual(got, []string{"command_name_invalid"}) {
			t.Fatalf("%s codes = %v", item, got)
		}
	}
	if len(envelope.Commands) != 1 || len(envelope.Diagnostics) != 5 {
		t.Fatalf("commands = %v, diagnostics = %+v", envelope.Commands, envelope.Diagnostics)
	}
}

func TestCommandProjectedNameChecks(t *testing.T) {
	t.Parallel()
	body := validHeader + "x\n"
	collision := load(t, "my.plug", []string{"guide", "My-Plug-Hello"}, file("hello.md", body), file("other.md", body))
	if got := codes(collision, "hello"); !reflect.DeepEqual(got, []string{"command_skill_name_collision"}) {
		t.Fatalf("collision codes = %v", got)
	}
	if _, ok := collision.Commands["other"]; !ok {
		t.Fatalf("a collision must not reject its siblings: %+v", collision.Diagnostics)
	}
	invalid := load(t, "bad.-name", nil, file("hello.md", "no header\n"))
	if got := codes(invalid, "hello"); !reflect.DeepEqual(got, []string{"command_frontmatter_invalid", "command_projected_name_invalid"}) {
		t.Fatalf("invalid projection codes = %v", got)
	}
	long := load(t, strings.Repeat("p", 60), nil, file("hello.md", body))
	if got := codes(long, "hello"); !reflect.DeepEqual(got, []string{"command_projected_name_too_long"}) {
		t.Fatalf("long projection codes = %v", got)
	}
}

func TestCommandLimitsComeFromConformanceDefaults(t *testing.T) {
	t.Parallel()
	limits := conformance.DefaultLimits()
	oversized := validHeader + strings.Repeat("x", limits.SkillBytes)
	if got := codes(load(t, "demo", nil, file("big.md", oversized)), "big"); !reflect.DeepEqual(got, []string{"command_frontmatter_invalid"}) {
		t.Fatalf("oversized codes = %v", got)
	}
	header := "---\ndescription: \"" + strings.Repeat("d", limits.FrontmatterBytes) + "\"\ndisable-model-invocation: true\n---\nx\n"
	if got := codes(load(t, "demo", nil, file("header.md", header)), "header"); !reflect.DeepEqual(got, []string{"command_frontmatter_invalid"}) {
		t.Fatalf("large frontmatter codes = %v", got)
	}
	truncated, err := commandext.WithCommands(context.Background(), domain.PackageEnvelope{}, commandext.Listing{Root: commandext.RootDirectory, Truncated: true, Entries: []commandext.Entry{file("ok.md", validHeader+"x\n")}})
	if err != nil {
		t.Fatal(err)
	}
	if len(truncated.Commands) != 0 || len(truncated.Diagnostics) != 1 || truncated.Diagnostics[0].Code != "commands_root_invalid" {
		t.Fatalf("truncated listing = %+v", truncated)
	}
	var many []commandext.Entry
	for i := 0; i <= limits.Diagnostics; i++ {
		many = append(many, commandext.Entry{Name: fmt.Sprintf("dir%03d", i), Kind: commandext.EntryDirectory})
	}
	capped := load(t, "demo", nil, many...)
	if len(capped.Diagnostics) != limits.Diagnostics || len(capped.InvalidCommands) != len(many) {
		t.Fatalf("diagnostics = %d, invalid = %d", len(capped.Diagnostics), len(capped.InvalidCommands))
	}
}

func TestCommandRootInvalidRejectsEverything(t *testing.T) {
	t.Parallel()
	envelope, err := commandext.WithCommands(context.Background(), domain.PackageEnvelope{Manifest: domain.PluginManifest{Name: "demo"}}, commandext.Listing{Root: commandext.RootInvalid})
	if err != nil {
		t.Fatal(err)
	}
	if envelope.HasInstallerRenderedComponents() || len(envelope.Diagnostics) != 1 || envelope.Diagnostics[0].Code != "commands_root_invalid" || envelope.Diagnostics[0].Severity != domain.SeverityError {
		t.Fatalf("envelope = %+v", envelope)
	}
}
