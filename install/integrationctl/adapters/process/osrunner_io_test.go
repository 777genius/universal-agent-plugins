package process

import (
	"bytes"
	"os/exec"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

func TestBindCommandPipesTeesStderr(t *testing.T) {
	t.Parallel()
	var live bytes.Buffer
	cmd := exec.Command("true")
	stdout, stderr := attachCommandPipes(cmd, ports.Command{Stderr: &live})
	if _, err := cmd.Stderr.Write([]byte("Receiving objects:  40% (1/2)\n")); err != nil {
		t.Fatal(err)
	}
	if live.String() != "Receiving objects:  40% (1/2)\n" {
		t.Fatalf("live stderr = %q", live.String())
	}
	if !bytes.Contains(stderr.Bytes(), []byte("Receiving objects:  40%")) {
		t.Fatalf("captured stderr = %q", stderr.Bytes())
	}
	if stdout == nil {
		t.Fatal("stdout pipe was not bound")
	}
}
