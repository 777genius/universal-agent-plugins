//go:build linux

package main

import (
	"encoding/json"
	"errors"
	"strconv"
)

const sessionMode = "session-only-v1"

func sessionFields(mode, original, extra string) string {
	if mode == sessionMode {
		return original + " " + extra
	}
	return original
}

// This sealed constant is the ONLY privileged Python code in session mode.
// It runs after unshare, before unchanged setpriv; imports no fixture code.
const loopbackBootstrap = `import fcntl, os, socket, struct, sys
def require(value):
    if not value:
        raise RuntimeError("loopback-bootstrap-refused")
require(sys.platform == "linux" and sys.byteorder == "little" and struct.calcsize("P") == 8)
require(os.uname().machine == "x86_64")
require(os.getuid() == os.geteuid() == os.getgid() == os.getegid() == 0)
require(len(sys.argv) >= 4 and sys.argv[1].isdigit())
parent = int(sys.argv[1])
namespace = os.stat("/proc/self/ns/net").st_ino
require(parent > 0 and namespace != parent)
require([name for _, name in socket.if_nameindex()] == ["lo"])
request = struct.pack("16sH22x", b"lo", 0)
with socket.socket(socket.AF_INET, socket.SOCK_DGRAM) as channel:
    def flags():
        raw = fcntl.ioctl(channel.fileno(), 0x8913, request)
        require(type(raw) is bytes and len(raw) == 40 and raw[:16] == b"lo" + b"\0" * 14)
        return struct.unpack_from("H", raw, 16)[0]
    original = flags()
    require(original & 8)
    if not original & 1:
        fcntl.ioctl(channel.fileno(), 0x8914, struct.pack("16sH22x", b"lo", original | 1))
    require(os.stat("/proc/self/ns/net").st_ino == namespace)
    require([name for _, name in socket.if_nameindex()] == ["lo"] and flags() & 9 == 9)
os.execve(sys.argv[2], sys.argv[2:], {"PATH": "/usr/bin:/bin", "LANG": "C.UTF-8"})
`

func sessionCommand(original []string, python string, parent uint64) []string {
	// original[3:] is the unchanged held setpriv/env/collector chain.
	result := append([]string{}, original[:3]...)
	result = append(result, python, "-I", "-c", loopbackBootstrap, strconv.FormatUint(parent, 10))
	return append(result, original[3:]...)
}

func validateSessionResult(got result, r request) error {
	if got.Mode != r.Mode {
		return errors.New("result-mode-refused")
	}
	if r.Mode != sessionMode {
		return nil
	}
	var p struct {
		Requests  int  `json:"requests"`
		Models    int  `json:"models_get"`
		Root      int  `json:"root_get"`
		Forbidden int  `json:"forbidden"`
		Settled   bool `json:"settled"`
	}
	if e := closed(got.Provider, &p, "requests models_get root_get forbidden settled"); e != nil {
		return e
	}
	if !p.Settled || p.Requests < 0 || p.Requests > 13 || p.Models < 0 || p.Root < 0 || p.Forbidden < 0 || p.Requests != p.Models+p.Root+p.Forbidden || got.Complete && p.Forbidden != 0 {
		return errors.New("provider-proof-refused")
	}
	return nil
}
