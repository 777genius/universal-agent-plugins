package defs

import "github.com/777genius/plugin-kit-ai/sdk/internal/runtime"

// This runtime-only profile is deliberately outside platformmeta's authoring
// targets. It supplies neither a scaffold nor a native install/profile adapter.
func vscodeLocalProfile() PlatformProfile {
	return PlatformProfile{
		Platform: "vscode-local", Status: runtime.StatusRuntimeSupported,
		PublicPackage: "vscodelocal", InternalPackage: "vscodelocal",
		InternalImport: "github.com/777genius/plugin-kit-ai/sdk/internal/platforms/vscodelocal",
		TransportModes: []runtime.TransportMode{runtime.ProcessMode},
		RuntimeOnly:    true,
	}
}
