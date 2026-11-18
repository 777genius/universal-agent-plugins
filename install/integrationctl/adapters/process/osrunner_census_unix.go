//go:build darwin || linux

package process

import (
	"bytes"
	"os"
	"strings"
)

// ownAncestry is this process and every ancestor reachable through parents.
// None of them can be a command left behind by an earlier invocation.
func ownAncestry(parents map[int]int) map[int]bool {
	ancestry := map[int]bool{}
	for pid := os.Getpid(); pid > 1 && !ancestry[pid]; pid = parents[pid] {
		ancestry[pid] = true
	}
	return ancestry
}

// nulFields splits a NUL-separated kernel string list.
func nulFields(body []byte) []string {
	body = bytes.TrimRight(body, "\x00")
	if len(body) == 0 {
		return nil
	}
	return strings.Split(string(body), "\x00")
}
