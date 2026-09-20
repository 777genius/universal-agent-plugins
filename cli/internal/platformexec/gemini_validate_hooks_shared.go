package platformexec

func geminiHookDiagnostic(code, path, message string) Diagnostic {
	return Diagnostic{
		Severity: SeverityFailure,
		Code:     code,
		Path:     path,
		Target:   "gemini",
		Message:  message,
	}
}
