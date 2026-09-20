package main

import (
	"strings"
	"testing"
)

func TestValidateUpdateArgs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		all     bool
		args    []string
		wantErr string
	}{
		{name: "all without name", all: true, args: nil},
		{name: "all with name", all: true, args: []string{"demo"}, wantErr: "update --all does not accept a name"},
		{name: "single name", all: false, args: []string{"demo"}},
		{name: "missing name", all: false, args: nil, wantErr: "update requires exactly one integration name unless --all is set"},
		{name: "too many names", all: false, args: []string{"one", "two"}, wantErr: "update requires exactly one integration name unless --all is set"},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := validateUpdateArgs(tc.all, tc.args)
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("validateUpdateArgs error = %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("validateUpdateArgs error = %v", err)
			}
		})
	}
}

func TestNewRootUpdateCommandValidatesAllWithoutName(t *testing.T) {
	t.Parallel()

	cmd := newRootUpdateCommand()
	if err := cmd.Flags().Set("all", "true"); err != nil {
		t.Fatalf("set --all: %v", err)
	}
	if err := cmd.Args(cmd, []string{"demo"}); err == nil || !strings.Contains(err.Error(), "update --all does not accept a name") {
		t.Fatalf("root update args error = %v", err)
	}
}
