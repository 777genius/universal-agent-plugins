//go:build linux

package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

const trustedScannerArchive = "3da60f749c61e2caca029a44a9ce422d570aef8c57f82ce51c411c8cec12f61b"

func inputDeadline(in inputRequest, phase string) (time.Time, error) {
	limits := map[string]time.Duration{"work": 420 * time.Second, "final": 440 * time.Second, "publish": 450 * time.Second}
	maximum := 480 * time.Second
	switch in.Mode {
	case "":
		if in.Trusted != nil || in.SourceTree != "" {
			return time.Time{}, fmt.Errorf("trusted inputs require explicit mode")
		}
	case trustedProjectMode:
		if in.Trusted == nil || in.ObserverRoot != "" || !commitPattern.MatchString(in.SourceTree) {
			return time.Time{}, fmt.Errorf("closed trusted request identity differs")
		}
		for _, item := range []struct{ path, sha string }{{in.Trusted.ToolsRoot, in.Trusted.ToolsManifestSHA256}, {in.Trusted.InstallerPath, in.Trusted.InstallerSHA256}, {in.Trusted.ProbePath, in.Trusted.ProbeSHA256}, {in.Trusted.ScannerPath, in.Trusted.ScannerSHA256}} {
			if !filepath.IsAbs(item.path) || filepath.Clean(item.path) != item.path || !shaPattern.MatchString(item.sha) {
				return time.Time{}, fmt.Errorf("trusted executable/tool identity differs")
			}
		}
		if in.Trusted.ScannerArchiveSHA256 != trustedScannerArchive {
			return time.Time{}, fmt.Errorf("pinned scanner archive differs")
		}
		limits = map[string]time.Duration{"work": 840 * time.Second, "final": 870 * time.Second, "publish": 880 * time.Second}
		maximum = 900 * time.Second
	default:
		return time.Time{}, fmt.Errorf("unknown controller mode")
	}
	limit, ok := limits[phase]
	if !ok {
		return time.Time{}, fmt.Errorf("unknown controller budget phase")
	}
	return deadlineAtMaximum(in.StartNS, limit, maximum)
}

func sealTrustedInputs(ctx context.Context, in inputRequest, base string) error {
	if _, err := inputDeadline(in, "work"); err != nil {
		return err
	}
	if _, err := copyTrustedTools(ctx, *in.Trusted, filepath.Join(base, "tools"), in.UID); err != nil {
		return err
	}
	if err := os.Mkdir(filepath.Join(base, "executables"), 0755); err != nil {
		return err
	}
	for _, item := range []struct{ name, path, sha string }{{"agentplugins", in.Trusted.InstallerPath, in.Trusted.InstallerSHA256}, {"native-probe", in.Trusted.ProbePath, in.Trusted.ProbeSHA256}, {"lintai", in.Trusted.ScannerPath, in.Trusted.ScannerSHA256}} {
		actual, err := copyOrdinary(ctx, item.path, filepath.Join(base, "executables", item.name), in.UID, true)
		if err != nil || actual != item.sha {
			return fmt.Errorf("trusted %s executable differs: %w", item.name, err)
		}
	}
	return nil
}

// Only the freshly created task network namespace is modified, before the
// unchanged capability/UID drop. No shell, command lookup or external interface.
const trustedLoopbackPrelude = `
import fcntl,os,socket,struct,sys
parent,*argv=sys.argv[1:]
if os.getuid()!=0 or os.geteuid()!=0 or os.stat("/proc/self/ns/net").st_ino==int(parent) or [name for _,name in socket.if_nameindex()]!=["lo"] or not argv or argv[0]!="/usr/bin/setpriv":raise SystemExit("owned loopback namespace required")
with socket.socket(socket.AF_INET,socket.SOCK_DGRAM) as control:
    name=struct.pack("16sH",b"lo",0)
    flags=struct.unpack("16sH",fcntl.ioctl(control.fileno(),0x8913,name)[:18])[1]
    fcntl.ioctl(control.fileno(),0x8914,struct.pack("16sH",b"lo",flags|1))
os.execve(argv[0],argv,{"PATH":"/usr/bin:/bin","LANG":"C.UTF-8"})
`

const trustedLoopbackVerification = `
import fcntl,struct
with socket.socket(socket.AF_INET,socket.SOCK_DGRAM) as control:
    flags=struct.unpack("16sH",fcntl.ioctl(control.fileno(),0x8913,struct.pack("16sH",b"lo",0))[:18])[1]
    if not flags&1:raise SystemExit("owned loopback interface is down")
with socket.socket(socket.AF_INET,socket.SOCK_STREAM) as listener:
    listener.bind(("127.0.0.1",0))
p["loopback_ready"]=True
`
