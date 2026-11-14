package runtimecheck

import "regexp"

const defaultNodeRuntimeTarget = "plugin/main.mjs"

var launcherTargetPatterns = []*regexp.Regexp{
	regexp.MustCompile(`\$ROOT/([^"\s]+\.(?:mjs|js|cjs))`),
	regexp.MustCompile(`%ROOT%/([^"\r\n]+\.(?:mjs|js|cjs))`),
}
