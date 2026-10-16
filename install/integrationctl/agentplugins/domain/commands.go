package domain

import "errors"

// ErrProjectionRefreshRequired means an installed tree holds command skills
// other than the ones the current rules render for it. Only an update
// applies the current rules; operations that keep the installed tree stop
// with this error instead.
var ErrProjectionRefreshRequired = errors.New("command projection rules changed since installation; run `agentplugins update`")

// ExtensionNamespace is the package directory reserved for agentplugins'
// own content. Authors write CommandsDir and LockedRuntimeDir below it; the
// installer writes ManagedStdioDir into the staged copy.
const (
	ExtensionNamespace = "io.github.777genius.agentplugins"
	CommandsDir        = "commands"
	LockedRuntimeDir   = "runtime"
	ManagedStdioDir    = "managed-stdio-v1"
)

const (
	// ExplicitInvocationField is the command field that asks for explicit
	// invocation. Every skill rendered from an explicit-only command carries
	// it as the YAML boolean true, whatever else its client needs.
	ExplicitInvocationField = "disable-model-invocation"
	// BoundaryCommand marks a diagnostic about one authored command or the
	// commands directory as a whole.
	BoundaryCommand FailureBoundary = "command"
	// ComponentSourceCommand marks a skill component the installer renders
	// from an authored command rather than copying it from skills/.
	ComponentSourceCommand = "command"
)

// Command is an authored command that passed every format check. It is
// delivered as a skill; the package never contains that skill's bytes.
type Command struct {
	// Name is the file stem, which is also the user-facing suffix of the
	// projected skill name.
	Name        string `json:"name"`
	Description string `json:"description"`
	// ExplicitOnly asks the client to run the command only when the user
	// invokes it. Without a client mechanism that enforces this, the command
	// is not delivered at all.
	ExplicitOnly bool `json:"explicit_only"`
	// Body is the Markdown after the frontmatter, with CRLF folded to LF.
	Body         []byte `json:"-"`
	RelativePath string `json:"relative_path"`
}

// FrontmatterField is one ordered YAML header field. Value is a string or a
// bool; renderers reject anything else.
type FrontmatterField struct {
	Key   string
	Value any
}

// RenderedSkill is a skill produced from a command, before it is written.
// Files are extra files below the skill directory, keyed by slash path.
type RenderedSkill struct {
	Frontmatter []FrontmatterField
	Body        []byte
	Files       map[string][]byte
}

// HasInstallerRenderedComponents reports whether the delivered tree depends
// on installer rendering rules as well as on the package bytes, so a newer
// installer can change it without a new package revision.
func (envelope PackageEnvelope) HasInstallerRenderedComponents() bool {
	return len(envelope.Commands)+len(envelope.InvalidCommands) > 0
}

// DeliversCommand reports whether a client receives a command as a skill. The
// client has to take skills and must not withhold commands in its registry
// entry. An explicit-only command also needs a client that enforces explicit
// invocation; it is withheld rather than delivered as a skill the model may
// pick on its own.
func DeliversCommand(command Command, definition ClientDefinition) bool {
	support := definition.Capabilities.SkillSupport
	return support != "" && support != SupportUnsupported && definition.CommandSupport != SupportUnsupported &&
		(!command.ExplicitOnly || definition.Traits.EnforcesExplicitSkillInvocation)
}

// DeliversSkills reports whether a client gets any skill from the package,
// authored or rendered from a command. Host preparation sizes skill support
// from this answer, so it has to agree with the planner.
func DeliversSkills(envelope PackageEnvelope, definition ClientDefinition) bool {
	if len(envelope.Skills) > 0 {
		return true
	}
	for _, command := range envelope.Commands {
		if DeliversCommand(command, definition) {
			return true
		}
	}
	return false
}
