package process

import (
	"io"
	"os/exec"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

func bindCommandPipes(command ports.Command) (stdout *synchronizedOutputBuffer, stderr *boundedDiagnosticBuffer, stderrWriter io.Writer) {
	stdout = newSynchronizedOutputBuffer(command.StdoutLimitBytes)
	stderr = newBoundedDiagnosticBuffer(32 * 1024)
	if command.Stderr == nil {
		return stdout, stderr, stderr
	}
	return stdout, stderr, io.MultiWriter(stderr, command.Stderr)
}

func attachCommandPipes(c *exec.Cmd, command ports.Command) (*synchronizedOutputBuffer, *boundedDiagnosticBuffer) {
	stdout, stderr, stderrWriter := bindCommandPipes(command)
	c.Stdout = stdout
	c.Stderr = stderrWriter
	return stdout, stderr
}
