package main

import (
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

func TestEnterprisePolicyWindowsProgramDataRegistration(t *testing.T) {
	tests := []struct {
		name             string
		value            string
		kind             uint32
		systemWindowsDir string
		want             string
	}{
		{"literal", `C:\ProgramData`, registry.SZ, "", `C:\ProgramData`},
		{"relocated", `D:\Company Data`, registry.SZ, "", `D:\Company Data`},
		{"literal-percent", `C:\literal%name%`, registry.SZ, "", `C:\literal%name%`},
		{"clean-literal", `D:\Company\.\Old\..\Data`, registry.SZ, "", `D:\Company\Data`},
		{"expandable-absolute", `D:\Company Data`, registry.EXPAND_SZ, "", `D:\Company Data`},
		{"system-drive", `%SystemDrive%\ProgramData`, registry.EXPAND_SZ, `C:\Windows`, `C:\ProgramData`},
		{"mixed-case-drive", `%sYsTeMdRiVe%\ProgramData`, registry.EXPAND_SZ, `D:\Windows`, `D:\ProgramData`},
		{"system-root", `%SYSTEMROOT%\..\ProgramData`, registry.EXPAND_SZ, `D:\Windows`, `D:\ProgramData`},
		{"windir", `%wInDiR%\Shared Data`, registry.EXPAND_SZ, `D:\Windows`, `D:\Windows\Shared Data`},
		{"root-installed-windows", `%SystemRoot%\ProgramData`, registry.EXPAND_SZ, `C:`, `C:\ProgramData`},
		{"root-installed-windir", `%windir%\ProgramData`, registry.EXPAND_SZ, `C:`, `C:\ProgramData`},
		{"root-installed-drive", `%SystemDrive%\ProgramData`, registry.EXPAND_SZ, `C:`, `C:\ProgramData`},
		{"empty", "", registry.SZ, `C:\Windows`, ""},
		{"relative", `ProgramData`, registry.SZ, `C:\Windows`, ""},
		{"drive-only", `C:`, registry.SZ, `C:\Windows`, ""},
		{"drive-relative", `C:ProgramData`, registry.SZ, `C:\Windows`, ""},
		{"root-relative", `\ProgramData`, registry.SZ, `C:\Windows`, ""},
		{"decoded-nul", "C:\\ProgramData\x00hidden", registry.SZ, `C:\Windows`, ""},
		{"wrong-type", `C:\ProgramData`, registry.DWORD, `C:\Windows`, ""},
		{"literal-token-is-relative", `%SystemDrive%\ProgramData`, registry.SZ, `C:\Windows`, ""},
		{"unknown-token", `C:\%UNKNOWN%\Data`, registry.EXPAND_SZ, `C:\Windows`, ""},
		{"programdata-token", `%ProgramData%`, registry.EXPAND_SZ, `C:\Windows`, ""},
		{"allusers-token", `%ALLUSERSPROFILE%`, registry.EXPAND_SZ, `C:\Windows`, ""},
		{"unmatched-token", `%SystemDrive`, registry.EXPAND_SZ, `C:\Windows`, ""},
		{"unmatched-trailing-percent", `C:\Data%`, registry.EXPAND_SZ, `C:\Windows`, ""},
		{"empty-token", `C:\%%\Data`, registry.EXPAND_SZ, `C:\Windows`, ""},
		{"expanded-drive-only", `%SystemDrive%`, registry.EXPAND_SZ, `C:\Windows`, ""},
		{"expanded-drive-relative", `%SystemDrive%ProgramData`, registry.EXPAND_SZ, `C:`, ""},
		{"missing-system-directory", `%SystemRoot%\Data`, registry.EXPAND_SZ, "", ""},
		{"relative-system-directory", `%SystemDrive%\Data`, registry.EXPAND_SZ, `Windows`, ""},
		{"drive-relative-system-directory", `%windir%\Data`, registry.EXPAND_SZ, `C:Windows`, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := resolveWindowsProgramData(tt.value, tt.kind, tt.systemWindowsDir)
			if tt.want == "" {
				if err == nil || got != "" {
					t.Fatalf("invalid registration resolved to %q, error %v", got, err)
				}
				return
			}
			if err != nil || got != tt.want {
				t.Fatalf("registration resolved to %q, error %v; want %q", got, err, tt.want)
			}
		})
	}
}

func TestEnterprisePolicyWindowsKnownFolderParity(t *testing.T) {
	root, err := windows.KnownFolderPath(windows.FOLDERID_ProgramData, 0)
	if err != nil {
		t.Fatalf("normal Windows ProgramData lookup: %v", err)
	}
	got, err := machinePolicyPath()
	if err != nil {
		t.Fatalf("machine installation policy lookup: %v", err)
	}
	want := filepath.Join(root, "AgentPlugins", "policy.yaml")
	if !strings.EqualFold(got, want) {
		t.Fatalf("machine policy = %q, normal KnownFolder policy = %q", got, want)
	}
}
