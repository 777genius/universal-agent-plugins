//go:build linux

package main

import (
	"strings"
	"testing"
)

func TestPublisherOriginalMountTopology(t *testing.T) {
	good := "28 1 8:1 / / rw,relatime - ext4 /dev/root rw\n"
	if _, err := publisherOriginalTopology([]byte(good), 28); err != nil {
		t.Fatal(err)
	}
	for name, body := range map[string]string{
		"descendant":         good + "29 28 8:2 / /usr/share/data rw - ext4 /dev/other rw\n",
		"escaped-descendant": good + "29 28 8:2 / /usr/share/with\\040space rw - ext4 /dev/other rw\n",
		"shared":             strings.Replace(good, " - ", " shared:1 - ", 1),
		"slave":              strings.Replace(good, " - ", " master:1 - ", 1),
		"unknown":            strings.Replace(good, " - ", " future:1 - ", 1),
		"duplicate":          good + good,
		"partial":            strings.TrimSuffix(good, "\n"),
		"unknown-escape":     strings.Replace(good, " / rw", " /bad\\999 rw", 1),
		"overflow":           strings.Repeat("x", publisherMountInfoLimit+1),
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := publisherOriginalTopology([]byte(body), 28); err == nil {
				t.Fatal("unsafe topology accepted")
			}
		})
	}
	if _, err := publisherOriginalTopology([]byte(good), 99); err == nil {
		t.Fatal("foreign mount accepted")
	}
}
