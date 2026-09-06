// Package hostdetect identifies which supported host product invoked the
// current hook process.
//
// Detection is layered: a valid explicit override always wins, then ordered
// registry signals match environment markers, then bounded top-level payload
// sniffing runs. Detection fails closed: when nothing matches, the result is
// PlatformUnknown, never a silent default host.
//
// Payload sniffing targets the stdin-JSON hooks wire formats. The legacy
// Codex notify payload (argv JSON with dashed keys such as "turn-id") is
// intentionally not recognized and yields PlatformUnknown; legacy notify
// consumers know their host statically.
package hostdetect
