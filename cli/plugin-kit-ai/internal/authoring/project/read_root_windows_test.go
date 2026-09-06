package project

import "testing"

func TestWindowsReadRootDriveRootPrefix(t *testing.T) {
	for _, tc := range []struct{ cwd, name, want string }{
		{`C:\`, ".", `C:\.`},
		{`C:\`, "demo", `C:\demo`},
		{`C:\`, `.\demo`, `C:\.\demo`},
		{`C:/`, "demo", `C:/demo`},
		{`D:\fixture`, `junction\..\demo`, `D:\fixture\junction\..\demo`},
	} {
		if got := prefixReadRoot(tc.cwd, tc.name); got != tc.want {
			t.Fatalf("%q + %q = %q; want %q", tc.cwd, tc.name, got, tc.want)
		}
	}
}
