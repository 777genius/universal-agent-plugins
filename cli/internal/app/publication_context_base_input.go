package app

import (
	"errors"
	"fmt"
	"strings"
)

func normalizePublicationRootInput(rootInput string) string {
	root := strings.TrimSpace(rootInput)
	if root == "" {
		return "."
	}
	return root
}

func validatePublicationTargetInput(targetInput, unsupportedMessage string) (string, error) {
	target := strings.TrimSpace(targetInput)
	switch target {
	case "codex-package", "claude":
		return target, nil
	default:
		return "", fmt.Errorf(unsupportedMessage, "codex-package", "claude")
	}
}

func validatePublicationDestInput(destInput, missingDestMessage string) (string, error) {
	dest := strings.TrimSpace(destInput)
	if dest == "" {
		return "", errors.New(missingDestMessage)
	}
	return dest, nil
}
