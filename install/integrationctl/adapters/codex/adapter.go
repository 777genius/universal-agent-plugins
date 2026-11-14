package codex

import "github.com/777genius/plugin-kit-ai/install/integrationctl/ports"

type Adapter struct {
	FS          ports.FileSystem
	ProjectRoot string
	UserHome    string
}
