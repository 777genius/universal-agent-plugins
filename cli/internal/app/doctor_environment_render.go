package app

import "fmt"

func buildDoctorEnvironmentLines(root string, specs []doctorToolSpec) []string {
	if len(specs) == 0 {
		return nil
	}

	lines, missing := buildDoctorEnvironmentDetailLines(root, specs)
	return appendDoctorEnvironmentHint(lines, missing)
}

func renderDoctorEnvironmentLine(root string, spec doctorToolSpec) (string, bool) {
	path, _, err := doctorFindBinary(spec.Commands)
	if err != nil {
		return "", false
	}
	line := fmt.Sprintf("  %s: ok (%s", spec.Label, path)
	if version := doctorVersion(root, path, spec.VersionArgs); version != "" {
		line += "; " + version
	}
	return finalizeDoctorEnvironmentLine(line), true
}
