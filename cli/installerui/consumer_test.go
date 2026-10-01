package installerui

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// This gate builds a real external main module with only the candidate CLI
// replace. It does not qualify an unpublished future no-replace revision.
func TestPublicTerminalConsumerCompile(t *testing.T) {
	base := os.Getenv("SELECTOR_CONSUMER_TMPDIR")
	if base == "" {
		base = t.TempDir()
	}
	root, err := os.MkdirTemp(base, "TEST-selector-consumer-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(root)
	module, err := filepath.Abs("..")
	if err != nil {
		t.Fatal(err)
	}
	gomod := "module example.com/selector-consumer\n\ngo 1.25.8\n\nrequire github.com/777genius/plugin-kit-ai/cli v0.0.0\n\nreplace github.com/777genius/plugin-kit-ai/cli => " + filepath.ToSlash(module) + "\n"
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte(gomod), 0600); err != nil {
		t.Fatal(err)
	}
	sums, err := os.ReadFile("../go.sum")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "go.sum"), sums, 0600); err != nil {
		t.Fatal(err)
	}
	sample := `package consumer
import (
 "context"
 "os"
 "github.com/777genius/plugin-kit-ai/cli/installerui"
)
var _ = installerui.Option{"alpha","Alpha"}
var _ = installerui.SelectRequest{"Choose",[]installerui.Option{{"alpha","Alpha"}},[]string{"alpha"}}
var _ = installerui.ConfirmRequest{"Apply?",[]string{"Full scope"},false}
var _ = installerui.Selection{[]string{"alpha"},true,false}
var _ = installerui.Confirmation{false,false}
var _ = installerui.Config{nil,nil,nil,nil}
func CorrectUse(ctx context.Context,in,out *os.File) error {
 terminal,err:=installerui.NewTerminal(installerui.TerminalConfig{Input:in,Output:out,Mode:installerui.ModeAuto,NoColor:true})
 if err!=nil{return err}
 menu,err:=terminal.PlainUI();if err!=nil{return err}
 _,err=menu.SelectOne(ctx,installerui.SelectRequest{Title:"Action",Options:[]installerui.Option{{ID:"inspect",Label:"Inspect"}}});if err!=nil{return err}
 _,err=terminal.SelectMany(ctx,installerui.MultiSelectRequest{SelectRequest:installerui.SelectRequest{Title:"Select",Options:[]installerui.Option{{ID:"alpha",Label:"Alpha"}}},MinSelected:0});if err!=nil{return err}
 _,err=terminal.Confirm(ctx,installerui.ConfirmRequest{Title:"Apply?",Default:false});return err
}
`
	if err := os.WriteFile(filepath.Join(root, "consumer.go"), []byte(sample), 0600); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("go", "test", "-mod=mod", "-count=1", "./...")
	cmd.Dir = root
	// Preserve only prepared build inputs and trusted tool paths, never auth env.
	for _, key := range []string{"PATH", "TMPDIR", "GOCACHE", "GOMODCACHE"} {
		cmd.Env = append(cmd.Env, key+"="+os.Getenv(key))
	}
	cmd.Env = append(cmd.Env, "HOME="+t.TempDir(), "GOTOOLCHAIN=local", "GOWORK=off", "GOPROXY=off", "GOSUMDB=off")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("external candidate module: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "example.com/selector-consumer") {
		t.Fatalf("no matched external module: %s", out)
	}
	t.Logf("external candidate CLI source, no transitive local replaces: %s", out)
}
