package main

import "testing"

func TestRootCommandExposesIntegrationShortAliases(t *testing.T) {
	t.Helper()

	tests := []struct {
		args []string
		want string
	}{
		{args: []string{"add"}, want: "add"},
		{args: []string{"update"}, want: "update"},
		{args: []string{"remove"}, want: "remove"},
		{args: []string{"repair"}, want: "repair"},
	}

	for _, tt := range tests {
		cmd, _, err := rootCmd.Find(tt.args)
		if err != nil {
			t.Fatalf("find %q: %v", tt.args[0], err)
		}
		if cmd == nil {
			t.Fatalf("find %q returned nil command", tt.args[0])
		}
		if cmd.Name() != tt.want {
			t.Fatalf("find %q resolved to %q", tt.args[0], cmd.Name())
		}
	}
}

func TestRootAddAliasAppliesByDefaultWhileIntegrationsAddStaysPlanFirst(t *testing.T) {
	t.Helper()

	rootAddCmd, _, err := rootCmd.Find([]string{"add"})
	if err != nil {
		t.Fatalf("find add: %v", err)
	}
	if rootAddCmd == nil {
		t.Fatal("find add returned nil command")
	}

	rootAddDryRun := rootAddCmd.Flags().Lookup("dry-run")
	if rootAddDryRun == nil {
		t.Fatal("root add is missing dry-run flag")
	}
	if rootAddDryRun.DefValue != "false" {
		t.Fatalf("root add dry-run default = %q, want false", rootAddDryRun.DefValue)
	}

	integrationsAddCmd, _, err := rootCmd.Find([]string{"integrations", "add"})
	if err != nil {
		t.Fatalf("find integrations add: %v", err)
	}
	if integrationsAddCmd == nil {
		t.Fatal("find integrations add returned nil command")
	}

	integrationsAddDryRun := integrationsAddCmd.Flags().Lookup("dry-run")
	if integrationsAddDryRun == nil {
		t.Fatal("integrations add is missing dry-run flag")
	}
	if integrationsAddDryRun.DefValue != "true" {
		t.Fatalf("integrations add dry-run default = %q, want true", integrationsAddDryRun.DefValue)
	}
}
