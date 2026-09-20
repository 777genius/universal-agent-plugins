package pluginmanifest

import "github.com/777genius/plugin-kit-ai/install/integrationctl/ports"

func inspectCanonicalSource(sourceRef string, resolved ports.ResolvedSource, target string) (SourceInspection, []Warning, error) {
	state, err := discoverPackageState(resolved.LocalPath)
	if err != nil {
		return SourceInspection{}, nil, err
	}
	inspection, err := inspectPackageContext(packageContext{
		root:            resolved.LocalPath,
		layout:          state.layout,
		graph:           state.graph,
		publication:     state.publication,
		selectedTargets: state.graph.Manifest.EnabledTargets(),
	})
	if err != nil {
		return SourceInspection{}, state.warnings, err
	}
	compatibility, err := buildSourceCompatibility(state.graph, state.graph.Manifest.EnabledTargets(), target, nil)
	if err != nil {
		return SourceInspection{}, state.warnings, err
	}
	return SourceInspection{
		RequestedSource:  sourceRef,
		ResolvedSource:   resolved.Resolved.Value,
		SourceKind:       resolved.Kind,
		SourceDigest:     resolved.SourceDigest,
		CanonicalPackage: true,
		OriginTargets:    state.graph.Manifest.EnabledTargets(),
		Inspection:       inspection,
		Compatibility:    compatibility,
	}, state.warnings, nil
}

func inspectImportedSource(sourceRef string, resolved ports.ResolvedSource, from string, target string, includeUserScope bool) (SourceInspection, []Warning, error) {
	prepared, err := prepareImportFromRoot(resolved.LocalPath, from, includeUserScope)
	if err != nil {
		return SourceInspection{}, prepared.Warnings, err
	}
	tmpRoot, cleanup, err := materializePreparedImport(prepared)
	if err != nil {
		return SourceInspection{}, prepared.Warnings, err
	}
	defer cleanup()
	state, err := discoverPackageState(tmpRoot)
	if err != nil {
		return SourceInspection{}, prepared.Warnings, err
	}
	inspection, err := inspectPackageContext(packageContext{
		root:            tmpRoot,
		layout:          state.layout,
		graph:           state.graph,
		publication:     state.publication,
		selectedTargets: state.graph.Manifest.EnabledTargets(),
	})
	if err != nil {
		return SourceInspection{}, append(prepared.Warnings, state.warnings...), err
	}
	compatibility, err := buildSourceCompatibility(state.graph, []string{prepared.ImportSource}, target, prepared.DroppedKinds)
	if err != nil {
		return SourceInspection{}, append(prepared.Warnings, state.warnings...), err
	}
	warnings := append([]Warning{}, prepared.Warnings...)
	warnings = append(warnings, state.warnings...)
	return SourceInspection{
		RequestedSource:     sourceRef,
		ResolvedSource:      resolved.Resolved.Value,
		SourceKind:          resolved.Kind,
		SourceDigest:        resolved.SourceDigest,
		CanonicalPackage:    false,
		ImportSource:        prepared.ImportSource,
		DetectedImportKinds: prepared.DetectedKinds,
		DroppedKinds:        prepared.DroppedKinds,
		OriginTargets:       []string{prepared.ImportSource},
		Inspection:          inspection,
		Compatibility:       compatibility,
	}, warnings, nil
}
