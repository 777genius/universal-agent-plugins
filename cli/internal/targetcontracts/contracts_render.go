package targetcontracts

import (
	"bytes"
	"strings"
	"text/tabwriter"
)

func Table(entries []Entry) []byte {
	var buf bytes.Buffer
	w := tabwriter.NewWriter(&buf, 0, 0, 2, ' ', 0)
	_, _ = w.Write([]byte("TARGET\tFAMILY\tCLASS\tLAUNCHER\tNOUN\tINSTALL\tDEV\tACTIVATION\tNATIVE ROOT\tPRODUCTION\tRUNTIME\tIMPORT\tRENDER\tVALIDATE\tPORTABLE\tTARGET-NATIVE\tNATIVE DOCS\tSURFACE TIERS\tMANAGED\tSUMMARY\n"))
	for _, entry := range entries {
		_, _ = w.Write([]byte(
			entry.Target + "\t" +
				entry.PlatformFamily + "\t" +
				entry.TargetClass + "\t" +
				entry.LauncherRequirement + "\t" +
				entry.TargetNoun + "\t" +
				entry.InstallModel + "\t" +
				entry.DevModel + "\t" +
				entry.ActivationModel + "\t" +
				entry.NativeRoot + "\t" +
				entry.ProductionClass + "\t" +
				entry.RuntimeContract + "\t" +
				yesNo(entry.ImportSupport) + "\t" +
				yesNo(entry.GenerateSupport) + "\t" +
				yesNo(entry.ValidateSupport) + "\t" +
				join(entry.PortableComponentKinds) + "\t" +
				join(entry.TargetComponentKinds) + "\t" +
				join(entry.NativeDocs) + "\t" +
				joinSurfaces(entry.NativeSurfaces) + "\t" +
				joinManagedArtifacts(entry.ManagedArtifactRules) + "\t" +
				entry.Summary + "\n"))
	}
	_ = w.Flush()
	return buf.Bytes()
}

func Markdown(entries []Entry) []byte {
	var b bytes.Buffer
	b.WriteString("# Target Support Matrix\n\n")
	b.WriteString("| Target | Platform Family | Target Class | Launcher | Target Noun | Install Model | Dev Model | Activation Model | Native Root | Production Class | Runtime Contract | Import | Generate | Validate | Portable Components | Target-native Components | Native Docs | Surface Tiers | Managed Artifacts | Summary |\n")
	b.WriteString("| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |\n")
	for _, entry := range entries {
		b.WriteString("| " + entry.Target + " | " + entry.PlatformFamily + " | " + entry.TargetClass + " | " + entry.LauncherRequirement + " | " + entry.TargetNoun + " | " + entry.InstallModel + " | " + entry.DevModel + " | " + entry.ActivationModel + " | " + entry.NativeRoot + " | " + entry.ProductionClass + " | " + entry.RuntimeContract + " | " + yesNo(entry.ImportSupport) + " | " + yesNo(entry.GenerateSupport) + " | " + yesNo(entry.ValidateSupport) + " | " + join(entry.PortableComponentKinds) + " | " + join(entry.TargetComponentKinds) + " | " + join(entry.NativeDocs) + " | " + joinSurfaces(entry.NativeSurfaces) + " | " + joinManagedArtifacts(entry.ManagedArtifactRules) + " | " + entry.Summary + " |\n")
	}
	return b.Bytes()
}

func join(items []string) string {
	if len(items) == 0 {
		return "-"
	}
	return strings.Join(items, ", ")
}

func joinSurfaces(items []Surface) string {
	if len(items) == 0 {
		return "-"
	}
	out := make([]string, 0, len(items))
	for _, item := range items {
		out = append(out, item.Kind+"="+item.Tier)
	}
	return strings.Join(out, ", ")
}

func joinManagedArtifacts(items []ManagedArtifact) string {
	if len(items) == 0 {
		return "-"
	}
	out := make([]string, 0, len(items))
	for _, item := range items {
		label := item.Path
		if strings.TrimSpace(item.Condition) != "" {
			label += " (" + item.Condition + ")"
		}
		out = append(out, label)
	}
	return strings.Join(out, ", ")
}

func yesNo(v bool) string {
	if v {
		return "yes"
	}
	return "no"
}
