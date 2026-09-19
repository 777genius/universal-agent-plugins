package providers

import "bytes"

type fixtureACPStdin struct {
	bytes.Buffer
	close func() error
}

func (stdin *fixtureACPStdin) Close() error {
	if stdin.close == nil {
		return nil
	}
	return stdin.close()
}
