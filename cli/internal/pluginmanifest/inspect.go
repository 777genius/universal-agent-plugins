package pluginmanifest

import (
	"github.com/777genius/plugin-kit-ai/cli/internal/publicationmodel"
	"github.com/777genius/plugin-kit-ai/cli/internal/publishschema"
	"github.com/777genius/plugin-kit-ai/cli/internal/targetcontracts"
)

type packageContext struct {
	root            string
	layout          authoredLayout
	graph           PackageGraph
	publication     publishschema.State
	selectedTargets []string
}

func loadPackageContext(root string, target string) (packageContext, []Warning, error) {
	state, err := discoverPackageState(root)
	if err != nil {
		return packageContext{}, nil, err
	}
	selected, err := state.graph.Manifest.SelectedTargets(target)
	if err != nil {
		return packageContext{}, state.warnings, err
	}
	return packageContext{
		root:            root,
		layout:          state.layout,
		graph:           state.graph,
		publication:     state.publication,
		selectedTargets: selected,
	}, state.warnings, nil
}

func inspectPackage(root string, target string) (Inspection, []Warning, error) {
	ctx, warnings, err := loadPackageContext(root, target)
	if err != nil {
		return Inspection{}, warnings, err
	}
	inspection, err := inspectPackageContext(ctx)
	if err != nil {
		return Inspection{}, warnings, err
	}
	return inspection, warnings, nil
}

func inspectPackageContext(ctx packageContext) (Inspection, error) {
	inspection := Inspection{
		Manifest:    ctx.graph.Manifest,
		Launcher:    ctx.graph.Launcher,
		Portable:    ctx.graph.Portable,
		SourceFiles: cloneStringSlice(ctx.graph.SourceFiles),
		Layout: InspectLayout{
			AuthoredRoot:      ctx.layout.Path(""),
			AuthoredInputs:    cloneStringSlice(ctx.graph.SourceFiles),
			BoundaryDocs:      boundaryDocsForLayout(ctx.layout),
			GeneratedGuide:    generatedGuideForLayout(ctx.layout),
			GeneratedByTarget: map[string][]string{},
		},
		Publication: publicationmodel.Build(ctx.graph, ctx.publication, ctx.selectedTargets),
	}
	generatedOutputs, err := generatedArtifactInventory(ctx.root, ctx.layout, ctx.graph, ctx.publication, ctx.selectedTargets)
	if err != nil {
		return Inspection{}, err
	}
	inspection.Layout.GeneratedOutputs = generatedOutputs

	for _, name := range ctx.selectedTargets {
		entry, ok := targetcontracts.Lookup(name)
		if !ok {
			continue
		}
		tc := ctx.graph.Targets[name]
		managedArtifacts, err := generatedArtifactInventory(ctx.root, ctx.layout, ctx.graph, ctx.publication, []string{name})
		if err != nil {
			return Inspection{}, err
		}
		inspection.Layout.GeneratedByTarget[name] = cloneStringSlice(managedArtifacts)
		inspection.Targets = append(inspection.Targets, InspectTarget{
			Target:              name,
			PlatformFamily:      entry.PlatformFamily,
			TargetClass:         entry.TargetClass,
			LauncherRequirement: entry.LauncherRequirement,
			TargetNoun:          entry.TargetNoun,
			ProductionClass:     entry.ProductionClass,
			RuntimeContract:     entry.RuntimeContract,
			InstallModel:        entry.InstallModel,
			DevModel:            entry.DevModel,
			ActivationModel:     entry.ActivationModel,
			NativeRoot:          entry.NativeRoot,
			PortableKinds:       cloneStringSlice(entry.PortableComponentKinds),
			TargetNativeKinds:   cloneStringSlice(discoveredTargetKinds(tc)),
			NativeDocPaths:      discoveredNativeDocPaths(tc),
			NativeSurfaces:      append([]targetcontracts.Surface(nil), entry.NativeSurfaces...),
			NativeSurfaceTiers:  cloneStringMap(entry.NativeSurfaceTiers),
			ManagedArtifacts:    managedArtifacts,
			UnsupportedKinds:    cloneStringSlice(unsupportedKinds(entry, ctx.graph, tc)),
		})
	}
	return inspection, nil
}
