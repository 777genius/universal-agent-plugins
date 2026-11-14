package pluginmanifest

import (
	"fmt"
	"os"

	"github.com/777genius/plugin-kit-ai/sdk/platformmeta"
)

func loadLauncherForTargets(root string, targets []string) (*Launcher, error) {
	requires := false
	for _, target := range targets {
		profile, ok := platformmeta.Lookup(target)
		if !ok {
			continue
		}
		if profile.Launcher.Requirement == platformmeta.LauncherRequired {
			requires = true
			break
		}
	}
	launcher, err := loadLauncher(root)
	if err == nil {
		return &launcher, nil
	}
	if os.IsNotExist(err) && !requires {
		return nil, nil
	}
	if os.IsNotExist(err) {
		layout, lerr := detectAuthoredLayout(root)
		if lerr != nil {
			return nil, lerr
		}
		return nil, fmt.Errorf("required launcher missing: %s", layout.Path(LauncherFileName))
	}
	return nil, err
}

func requiresLauncherForTarget(target string) bool {
	profile, ok := platformmeta.Lookup(target)
	return ok && profile.Launcher.Requirement == platformmeta.LauncherRequired
}
