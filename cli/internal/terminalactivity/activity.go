// Package terminalactivity displays one bounded operation without owning input.
package terminalactivity

import (
	"fmt"
	"io"
	"sync"
	"time"
)

// Start returns an idempotent stop that joins the renderer before the caller
// prints an error or prompt. Plain output contains no terminal escape codes.
func Start(writer io.Writer, label string, live bool, width func() int, renderers ...func(int, string, string, int) string) func() {
	if !live {
		_, _ = fmt.Fprintln(writer, label+"…")
		return func() {}
	}
	started := time.Now()
	frames := []rune("⠋⠙⠹⠸⠼⠴⠦⠧⠇⠏")
	frame := 0
	paint := func() {
		elapsed := fmt.Sprintf("%ds", int(time.Since(started).Seconds()))
		line := []rune(fmt.Sprintf("%c %s · %s", frames[frame%len(frames)], label, elapsed))
		columns := 80
		if width != nil {
			columns = width()
		}
		if columns < 2 {
			columns = 2
		}
		if len(line) >= columns {
			line = line[:columns-1]
		}
		view := string(line)
		if len(renderers) > 0 {
			view = renderers[0](frame, label, elapsed, columns)
		}
		_, _ = fmt.Fprintf(writer, "\r\x1b[2K%s", view)
		frame++
	}
	paint()
	stop, done := make(chan struct{}), make(chan struct{})
	go func() {
		defer close(done)
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-stop:
				_, _ = io.WriteString(writer, "\r\x1b[2K")
				return
			case <-ticker.C:
				paint()
			}
		}
	}()
	var once sync.Once
	return func() { once.Do(func() { close(stop) }); <-done }
}
