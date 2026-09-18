package clientdetect

import (
	"os"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
)

// host builds the probing environment the client adapters observe through. The
// surface constructors live in the clients contract, so the evidence strings
// stay identical for the detector and for the contract harness.
func (detector Detector) host() clients.Host {
	readDir := detector.ReadDir
	if readDir == nil {
		readDir = os.ReadDir
	}
	return clients.NewHost(clients.HostProbes{
		HomeDir:               detector.HomeDir,
		GOOS:                  detector.GOOS,
		Environment:           detector.Environment,
		SystemApplicationsDir: detector.SystemApplicationsDir,
		WindowsProgramFiles:   detector.WindowsProgramFiles,
		LinuxApplicationDirs:  detector.LinuxApplicationDirs,
		LookPath:              detector.LookPath,
		Lstat:                 detector.Lstat,
		ReadDir:               readDir,
	})
}
