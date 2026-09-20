package capabilities

import (
	"bytes"
	"encoding/json"
	"strings"
	"text/tabwriter"

	"github.com/777genius/plugin-kit-ai/cli/internal/targetcontracts"
	pluginkitai "github.com/777genius/plugin-kit-ai/sdk"
)

type Entry struct {
	Platform        string   `json:"platform"`
	Event           string   `json:"event"`
	Status          string   `json:"status"`
	Maturity        string   `json:"maturity"`
	ContractClass   string   `json:"contract_class"`
	V1Target        bool     `json:"v1_target"`
	InvocationKind  string   `json:"invocation_kind"`
	Carrier         string   `json:"carrier"`
	TransportModes  []string `json:"transport_modes"`
	ScaffoldSupport bool     `json:"scaffold_support"`
	ValidateSupport bool     `json:"validate_support"`
	Capabilities    []string `json:"capabilities"`
	Summary         string   `json:"summary"`
	LiveTestProfile string   `json:"live_test_profile,omitempty"`
}

type TargetEntry = targetcontracts.Entry

func All() []Entry {
	return fromSupport(pluginkitai.Supported())
}

func ByPlatform(name string) []Entry {
	all := All()
	name = strings.ToLower(strings.TrimSpace(name))
	if name == "" {
		return all
	}
	out := make([]Entry, 0, len(all))
	for _, entry := range all {
		if entry.Platform == name {
			out = append(out, entry)
		}
	}
	return out
}

func JSON(entries []Entry) ([]byte, error) {
	return json.MarshalIndent(entries, "", "  ")
}

func TargetAll() []TargetEntry {
	return targetcontracts.All()
}

func TargetByPlatform(name string) []TargetEntry {
	return targetcontracts.ByTarget(name)
}

func TargetJSON(entries []TargetEntry) ([]byte, error) {
	return targetcontracts.JSON(entries)
}

func TargetTable(entries []TargetEntry) []byte {
	return targetcontracts.Table(entries)
}

func Table(entries []Entry) []byte {
	var buf bytes.Buffer
	w := tabwriter.NewWriter(&buf, 0, 0, 2, ' ', 0)
	_, _ = w.Write([]byte("PLATFORM\tEVENT\tSTATUS\tMATURITY\tCONTRACT\tV1\tINVOCATION\tCARRIER\tTRANSPORT\tSCAFFOLD\tVALIDATE\tLIVE_TEST\tCAPABILITIES\tSUMMARY\n"))
	for _, entry := range entries {
		_, _ = w.Write([]byte(entry.Platform + "\t" + entry.Event + "\t" + entry.Status + "\t" + entry.Maturity + "\t" + entry.ContractClass + "\t" + yesNo(entry.V1Target) + "\t" + entry.InvocationKind + "\t" + entry.Carrier + "\t" + join(entry.TransportModes) + "\t" + yesNo(entry.ScaffoldSupport) + "\t" + yesNo(entry.ValidateSupport) + "\t" + dashIfEmpty(entry.LiveTestProfile) + "\t" + join(entry.Capabilities) + "\t" + entry.Summary + "\n"))
	}
	_ = w.Flush()
	return buf.Bytes()
}

func fromSupport(entries []pluginkitai.SupportEntry) []Entry {
	out := make([]Entry, 0, len(entries))
	for _, entry := range entries {
		out = append(out, Entry{
			Platform:        string(entry.Platform),
			Event:           string(entry.Event),
			Status:          string(entry.Status),
			Maturity:        string(entry.Maturity),
			ContractClass:   contractClass(string(entry.Status), string(entry.Maturity)),
			V1Target:        entry.V1Target,
			InvocationKind:  string(entry.InvocationKind),
			Carrier:         string(entry.Carrier),
			TransportModes:  transportModes(entry.TransportModes),
			ScaffoldSupport: entry.ScaffoldSupport,
			ValidateSupport: entry.ValidateSupport,
			Capabilities:    capabilities(entry.Capabilities),
			Summary:         entry.Summary,
			LiveTestProfile: entry.LiveTestProfile,
		})
	}
	return out
}

func contractClass(status, maturity string) string {
	if status == "runtime_supported" {
		switch maturity {
		case "stable":
			return "production-ready"
		case "experimental":
			return "public-experimental"
		default:
			return "runtime-supported but not stable"
		}
	}
	switch maturity {
	case "experimental":
		return "public-experimental"
	default:
		return "public-beta"
	}
}

func capabilities(in []pluginkitai.CapabilityID) []string {
	out := make([]string, 0, len(in))
	for _, cap := range in {
		out = append(out, string(cap))
	}
	return out
}

func transportModes(in []pluginkitai.TransportMode) []string {
	out := make([]string, 0, len(in))
	for _, mode := range in {
		out = append(out, string(mode))
	}
	return out
}

func join(parts []string) string {
	if len(parts) == 0 {
		return "-"
	}
	out := parts[0]
	for i := 1; i < len(parts); i++ {
		out += "," + parts[i]
	}
	return out
}

func yesNo(v bool) string {
	if v {
		return "yes"
	}
	return "no"
}

func dashIfEmpty(v string) string {
	if strings.TrimSpace(v) == "" {
		return "-"
	}
	return v
}
