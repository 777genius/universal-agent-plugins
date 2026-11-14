package claude

import "github.com/777genius/plugin-kit-ai/install/integrationctl/ports"

type Adapter struct {
	Runner      ports.ProcessRunner
	FS          ports.FileSystem
	ProjectRoot string
	UserHome    string
}
