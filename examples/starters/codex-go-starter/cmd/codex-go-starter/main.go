package main

import (
	"os"

	pluginkitai "github.com/777genius/plugin-kit-ai/sdk"
	"github.com/777genius/plugin-kit-ai/sdk/codex"
)

func main() {
	app := pluginkitai.New(pluginkitai.Config{Name: "codex-go-starter"})
	app.Codex().OnNotify(func(e *codex.NotifyEvent) *codex.Response {
		_ = e
		return codex.Continue()
	})
	os.Exit(app.Run())
}
