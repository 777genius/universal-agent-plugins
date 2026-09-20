package validate

import "strings"

func normalizeReport(report Report) Report {
	report.Checks = cloneStrings(report.Checks)
	report.Warnings = cloneWarnings(report.Warnings)
	report.Failures = cloneFailures(report.Failures)
	return report
}

func cloneStrings(items []string) []string {
	if len(items) == 0 {
		return []string{}
	}
	return append([]string{}, items...)
}

func cloneWarnings(items []Warning) []Warning {
	if len(items) == 0 {
		return []Warning{}
	}
	return append([]Warning{}, items...)
}

func cloneFailures(items []Failure) []Failure {
	if len(items) == 0 {
		return []Failure{}
	}
	return append([]Failure{}, items...)
}

func targetOrAll(platform string) string {
	if strings.TrimSpace(platform) == "" {
		return "all"
	}
	return platform
}
