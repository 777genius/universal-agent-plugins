package app

func buildDoctorEnvironmentDetailLines(root string, specs []doctorToolSpec) ([]string, bool) {
	lines := []string{"Environment:"}
	missing := false
	for _, spec := range specs {
		line, found := renderDoctorEnvironmentLine(root, spec)
		if !found {
			lines = append(lines, "  "+doctorMissingLine(spec))
			missing = true
			continue
		}
		lines = append(lines, line)
	}
	return lines, missing
}

func appendDoctorEnvironmentHint(lines []string, missing bool) []string {
	if missing {
		lines = append(lines, "  Hint: "+doctorPATHHint())
	}
	return lines
}

func finalizeDoctorEnvironmentLine(line string) string {
	return line + ")"
}
