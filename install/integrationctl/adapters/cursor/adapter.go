package cursor

import "github.com/777genius/plugin-kit-ai/install/integrationctl/ports"

type Adapter struct {
	FS          ports.FileSystem
	SafeMutator ports.SafeFileMutator
	ProjectRoot string
	UserHome    string
}
