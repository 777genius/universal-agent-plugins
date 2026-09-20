package pluginmanifest

import (
	"fmt"
	"path/filepath"

	"github.com/777genius/plugin-kit-ai/cli/internal/platformexec"
	"github.com/777genius/plugin-kit-ai/cli/internal/pluginmodel"
	"github.com/777genius/plugin-kit-ai/sdk/platformmeta"
)

func discoverTarget(root string, layout authoredLayout, target string) (TargetComponents, error) {
	profile, ok := platformmeta.Lookup(target)
	if !ok {
		return TargetComponents{}, fmt.Errorf("unsupported target %q", target)
	}
	state := newTargetComponents(target)
	docKinds := map[string]struct{}{}
	mirrorKinds := map[string]struct{}{}
	for _, spec := range profile.ManagedArtifacts {
		if spec.Kind == platformmeta.ManagedArtifactMirror {
			mirrorKinds[spec.ComponentKind] = struct{}{}
		}
	}
	for _, spec := range profile.NativeDocs {
		docKinds[spec.Kind] = struct{}{}
		path := pluginmodel.RebaseAuthoredPath(spec.Path, layout.Path(""))
		if fileExists(filepath.Join(root, path)) {
			state.SetDoc(spec.Kind, path)
			if _, ok := mirrorKinds[spec.Kind]; ok {
				state.AddComponent(spec.Kind, path)
			}
		}
	}
	for _, kind := range profile.Contract.TargetComponentKinds {
		if _, isDoc := docKinds[kind]; isDoc {
			continue
		}
		dir := layout.Path(filepath.Join("targets", target, kind))
		state.AddComponent(kind, discoverFiles(root, dir, nil)...)
	}
	adapter, ok := platformexec.Lookup(target)
	if !ok {
		return TargetComponents{}, fmt.Errorf("unsupported target %q", target)
	}
	if err := adapter.RefineDiscovery(root, &state); err != nil {
		return TargetComponents{}, err
	}
	return state, nil
}
