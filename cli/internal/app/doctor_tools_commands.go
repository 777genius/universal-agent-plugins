package app

import "strings"

func normalizeDoctorCommands(commands []string) []string {
	out := make([]string, 0, len(commands))
	for _, command := range commands {
		command = strings.TrimSpace(command)
		if command != "" {
			out = append(out, command)
		}
	}
	return out
}
