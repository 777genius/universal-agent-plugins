package report

const AuthoringSchemaVersion = 1

// Public embeds the existing policy report, with independently versioned command
// semantics. Inspection contains display values, never retained parser objects.
type CommandHelp struct {
	Use      string   `json:"use"`
	Flags    []string `json:"flags"`
	Guidance string   `json:"guidance"`
}

type Public struct {
	Help *CommandHelp `json:"help,omitempty"`
	Report
	AuthoringSchemaVersion int         `json:"authoring_schema_version"`
	EngineVersion          string      `json:"engine_version"`
	Requested              Requested   `json:"requested"`
	Effects                Effects     `json:"effects"`
	NextActions            []Action    `json:"next_actions"`
	Inspection             *Inspection `json:"inspection,omitempty"`
	Surface                []string    `json:"commands,omitempty"`
	WithheldPathIDs        []string    `json:"withheld_path_ids,omitempty"`
}
type Requested struct {
	Operation string `json:"operation"`
	Mode      string `json:"mode"`
}
type Effects struct {
	// Attempted means the selected service was entered, not that a write occurred.
	Attempted bool `json:"attempted"`
	Committed bool `json:"committed"`
}
type Action struct {
	Code      string `json:"code"`
	Operation string `json:"operation,omitempty"`
	Message   string `json:"message"`
}
type Inspection struct {
	Name       string             `json:"name,omitempty"`
	Version    string             `json:"version,omitempty"`
	Schema     string             `json:"schema"`
	Components []DisplayComponent `json:"components"`
	Truncated  bool               `json:"truncated,omitempty"`
}
type DisplayComponent struct {
	ID             string `json:"id"`
	Type           string `json:"type"`
	Name           string `json:"name,omitempty"`
	Namespace      string `json:"namespace,omitempty"`
	Executable     string `json:"executable,omitempty"`
	ExecutableKind string `json:"executable_kind,omitempty"`
}

// AddOperationError does not evaluate package policies. Syntax, help and
// cancellation before decoding supply no facts about package validity or safety.
func (r *Report) AddOperationError(code, action string) {
	r.Error = &Error{Code: code, Action: action}
	r.add(Finding{Code: code, Layer: "operation", Rule: "authoring/command", Severity: "error"})
	r.finish()
}

// PublicResult is a value projection: it does not rewrite installer facts or the
// private vertical-slice report, including its diagnostic IDs and policy states.
func (r Report) PublicResult(operation, mode string, attempted bool, surface []string) Public {
	r.Command, r.Mode = operation, mode
	replacements := r.normalizePublicFindings()
	r.remapPublicAssessments(replacements)
	r.finish()
	p := Public{Report: r, AuthoringSchemaVersion: AuthoringSchemaVersion, EngineVersion: r.Engine,
		Requested: Requested{operation, mode}, Effects: Effects{attempted, r.Committed}, NextActions: []Action{}, Inspection: r.Display, Surface: surface}
	p.projectPublicPaths(r.Paths)
	p.projectPublicActions(replacements)
	return p
}
