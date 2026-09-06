// Package hostdetect identifies which supported host product invoked the
// current hook process.
//
// Detection is layered: a valid explicit override always wins, then ordered
// registry signals match environment markers, then bounded top-level payload
// sniffing runs. Detection fails closed: when nothing matches, the result is
// PlatformUnknown, never a silent default host.
package hostdetect
