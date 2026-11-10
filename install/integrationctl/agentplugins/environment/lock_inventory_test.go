package environment

import (
	"reflect"
	"strings"
	"testing"
)

func TestExecutableInventoryPortableNames(t *testing.T) {
	for _, files := range [][]string{
		nil, {}, {"bin/omega", "bin/alpha"}, {"scripts/工具"},
		{strings.Repeat("long-component/", 63) + strings.Repeat("a", 255)},
	} {
		before := append([]string(nil), files...)
		if err := ValidateExecutableInventory(files); err != nil {
			t.Fatalf("valid inventory rejected: %v", err)
		}
		if len(files) != 0 && !reflect.DeepEqual(before, files) {
			t.Fatal("validation rewrote inventory order")
		}
	}
	for _, files := range [][]string{
		{""}, {"."}, {".."}, {"../outside"}, {"bin/../outside"}, {"/absolute"},
		{"bin//helper"}, {"bin/"}, {`bin\helper`}, {"C:/helper"}, {"bin/COM1.txt"},
		{"bin/tool."}, {"bin/tool "}, {"bin/\x00tool"}, {"bin/\xfftool"}, {"bin/e\u0301"},
		{".git/hooks/post-checkout"}, {strings.Repeat("a/", 64) + "helper"},
		{"bin/helper", "bin/helper"}, {"bin/helper", "bin/HELPER"},
		{"bin/Straße", "bin/STRASSE"},
	} {
		if err := ValidateExecutableInventory(files); err == nil {
			t.Fatalf("invalid inventory admitted: %q", files)
		}
	}
}
