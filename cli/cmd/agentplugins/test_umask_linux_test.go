//go:build linux

package main

import "syscall"

// Native adapters such as Grok refuse an input file that is writable by its
// group or by others, and the fixtures create files with the umask of whoever
// runs the tests. Pin the usual 022 so the tests do not depend on the runner,
// where a user-private-group umask of 002 is the default.
func init() { syscall.Umask(0o022) }
