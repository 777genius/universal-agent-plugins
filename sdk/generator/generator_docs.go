package generator

import "strings"

func renderSupportMatrix(m model) string {
	var b strings.Builder
	b.WriteString("# Generated Support Matrix\n\n")
	b.WriteString("This generated table is the canonical per-event runtime reference for shipped support claims. Use it as exact contract data, not as front-door positioning. Package, extension, and repo-managed integration lanes remain summarized in SUPPORT.md and the target support matrix.\n\n")
	b.WriteString("| Platform | Event | Status | Maturity | Contract Class | V1 Target | Invocation | Carrier | Transport Modes | Scaffold | Validate | Capabilities | Live Test | Summary |\n")
	b.WriteString("|----------|-------|--------|----------|----------------|-----------|------------|---------|-----------------|----------|----------|--------------|-----------|---------|\n")
	for _, e := range m.events {
		b.WriteString(renderSupportMatrixRow(m, e))
	}
	return b.String()
}

func renderTargetSupportMatrix(m model) string {
	var b strings.Builder
	b.WriteString("# Target Support Matrix\n\n")
	b.WriteString("| Target | Platform Family | Target Class | Launcher | Target Noun | Install Model | Dev Model | Activation Model | Native Root | Production Class | Runtime Contract | Import | Generate | Validate | Portable Components | Target-native Components | Native Docs | Surface Tiers | Managed Artifacts | Summary |\n")
	b.WriteString("| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |\n")
	for _, profile := range scaffoldTargetProfiles(m) {
		b.WriteString(renderTargetSupportMatrixRow(profile))
	}
	return b.String()
}
