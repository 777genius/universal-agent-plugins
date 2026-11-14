package main

import (
	"os"

	pluginkitai "github.com/777genius/plugin-kit-ai/sdk"
	"github.com/777genius/plugin-kit-ai/sdk/claude"
)

func main() {
	app := pluginkitai.New(pluginkitai.Config{Name: "plugin-kit-ai-smoke"})
	app.Claude().OnStop(func(e *claude.StopEvent) *claude.Response {
		_ = e // smoke: allow stop
		return claude.Allow()
	})
	os.Exit(app.Run())
}
