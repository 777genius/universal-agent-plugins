package main

import "io"

// Count bytes emitted through the real CLI stdout. Some command handlers
// already emit complete failure JSON before returning an error; never append
// a second JSON document to those responses.
type agentpluginsOutputCounter struct {
	destination io.Writer
	written     int
}

func (counter *agentpluginsOutputCounter) Write(data []byte) (int, error) {
	n, err := counter.destination.Write(data)
	counter.written += n
	return n, err
}
