//go:build linux

package process

import (
	"fmt"
	"strings"
)

// The existing parser has already bounded fields to 10..16. Report only finite
// classes and structural counts; mount paths, sources and token values stay private.
func readonlyChildMountShapeRefusal(fields []string, expected int) error {
	observed, tail := -1, -1
	var empty uint16
	for i, field := range fields {
		if field == "" {
			empty |= 1 << i
		}
		if i >= 6 && observed == -1 && field == "-" {
			observed = i
		}
	}
	end := len(fields)
	if observed >= 6 {
		end, tail = observed, len(fields)-observed-1
	}
	classes := make([]string, 0, end-6)
	for _, field := range fields[6:end] {
		classes = append(classes, readonlyChildMountTagClass(field))
	}
	return fmt.Errorf("readonly child mount propagation or filesystem fields differ: field_count=%d expected_separator=%d observed_separator=%d tail_count=%d empty_fields=0x%04x optional_classes=[%s] role=%s",
		len(fields), expected, observed, tail, empty, strings.Join(classes, ","), readonlyChildMountRole(fields[4]))
}

func readonlyChildMountTagClass(field string) string {
	switch field {
	case "":
		return "empty"
	case "unbindable", "idmapped":
		return field
	}
	kind, _, found := strings.Cut(field, ":")
	if found {
		switch kind {
		case "shared", "master", "propagate_from":
			return kind
		}
	}
	return "unknown"
}

func readonlyChildMountRole(path string) string {
	switch path {
	case "/":
		return "root"
	case "/proc":
		return "proc"
	case "/proc/sys", "/proc/sysrq-trigger", "/proc/irq", "/proc/bus":
		return "proc-cover"
	case "/dev":
		return "dev"
	case "/dev/pts":
		return "devpts"
	case "/dev/null", "/dev/zero", "/dev/full", "/dev/random", "/dev/urandom", "/dev/tty":
		return "device"
	case "/usr", "/bin", "/lib", "/lib64":
		return "runtime"
	case readonlyBootstrapPath:
		return "bootstrap"
	default:
		return "input-or-unknown"
	}
}
