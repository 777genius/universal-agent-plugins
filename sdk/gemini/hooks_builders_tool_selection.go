package gemini

// BeforeToolSelectionContinue returns an explicit no-op BeforeToolSelection response.
func BeforeToolSelectionContinue() *BeforeToolSelectionResponse {
	return &BeforeToolSelectionResponse{}
}

// BeforeToolSelectionQuiet suppresses Gemini's internal hook metadata for the
// current tool-selection step without changing toolConfig.
func BeforeToolSelectionQuiet() *BeforeToolSelectionResponse {
	return &BeforeToolSelectionResponse{SuppressOutput: true}
}

// BeforeToolSelectionConfig applies a tool selection mode. Gemini currently
// accepts allowedFunctionNames only together with ANY mode.
func BeforeToolSelectionConfig(mode ToolMode, allowedFunctionNames ...string) *BeforeToolSelectionResponse {
	return &BeforeToolSelectionResponse{
		Mode:                 mode,
		AllowedFunctionNames: append([]string(nil), allowedFunctionNames...),
	}
}

// BeforeToolSelectionAllowOnly restricts Gemini tool selection to the provided
// allowlist by using ANY mode, which is the vendor-accepted shape for
// allowedFunctionNames.
func BeforeToolSelectionAllowOnly(allowedFunctionNames ...string) *BeforeToolSelectionResponse {
	return BeforeToolSelectionConfig(ToolModeAny, allowedFunctionNames...)
}

// BeforeToolSelectionForceAny requires Gemini to pick at least one tool and
// optionally narrows the candidate set with an allowlist.
func BeforeToolSelectionForceAny(allowedFunctionNames ...string) *BeforeToolSelectionResponse {
	return BeforeToolSelectionConfig(ToolModeAny, allowedFunctionNames...)
}

// BeforeToolSelectionForceAuto explicitly restores AUTO tool mode. Gemini does
// not currently accept allowedFunctionNames outside ANY mode, so any optional
// allowlist arguments are ignored.
func BeforeToolSelectionForceAuto(allowedFunctionNames ...string) *BeforeToolSelectionResponse {
	return BeforeToolSelectionConfig(ToolModeAuto)
}

// BeforeToolSelectionDisableAll disables all tools for the current decision step.
func BeforeToolSelectionDisableAll() *BeforeToolSelectionResponse {
	return BeforeToolSelectionConfig(ToolModeNone)
}
