package app

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/777genius/plugin-kit-ai/cli/internal/skills/domain"
)

func validateSkillExecution(root, skillPath, name string, doc domain.SkillDocument) []ValidationFailure {
	if doc.Spec.ExecutionMode == domain.ExecutionCommand {
		return validateCommandSkill(root, skillPath, name, doc)
	}
	return validateDocsOnlySkill(skillPath, doc)
}

func validateCommandSkill(root, skillPath, name string, doc domain.SkillDocument) []ValidationFailure {
	var failures []ValidationFailure
	if strings.TrimSpace(doc.Spec.Command) == "" {
		failures = append(failures, ValidationFailure{Path: skillPath, Message: "execution_mode=command requires command"})
	}
	failures = append(failures, validateWorkingDir(root, skillPath, name, doc.Spec.WorkingDir)...)
	failures = append(failures, validateTimeout(skillPath, doc.Spec.Timeout)...)
	switch doc.Spec.Runtime {
	case domain.RuntimeGo, domain.RuntimeShell, domain.RuntimePython, domain.RuntimeNode, domain.RuntimeDeno, domain.RuntimeExternal, domain.RuntimeGeneric:
	default:
		failures = append(failures, ValidationFailure{Path: skillPath, Message: fmt.Sprintf("execution_mode=command requires valid runtime (got %q)", doc.Spec.Runtime)})
	}
	return failures
}

func validateDocsOnlySkill(skillPath string, doc domain.SkillDocument) []ValidationFailure {
	var failures []ValidationFailure
	if strings.TrimSpace(doc.Spec.Command) != "" {
		failures = append(failures, ValidationFailure{Path: skillPath, Message: "execution_mode=docs_only must not define command"})
	}
	if len(doc.Spec.Args) > 0 {
		failures = append(failures, ValidationFailure{Path: skillPath, Message: "execution_mode=docs_only must not define args"})
	}
	if strings.TrimSpace(string(doc.Spec.Runtime)) != "" {
		failures = append(failures, ValidationFailure{Path: skillPath, Message: "execution_mode=docs_only must not define runtime"})
	}
	if strings.TrimSpace(doc.Spec.WorkingDir) != "" {
		failures = append(failures, ValidationFailure{Path: skillPath, Message: "execution_mode=docs_only must not define working_dir"})
	}
	if strings.TrimSpace(doc.Spec.Timeout) != "" {
		failures = append(failures, ValidationFailure{Path: skillPath, Message: "execution_mode=docs_only must not define timeout"})
	}
	if doc.Spec.SafeToRetry != nil {
		failures = append(failures, ValidationFailure{Path: skillPath, Message: "execution_mode=docs_only must not define safe_to_retry"})
	}
	if doc.Spec.WritesFiles != nil {
		failures = append(failures, ValidationFailure{Path: skillPath, Message: "execution_mode=docs_only must not define writes_files"})
	}
	if doc.Spec.ProducesJSON != nil {
		failures = append(failures, ValidationFailure{Path: skillPath, Message: "execution_mode=docs_only must not define produces_json"})
	}
	return failures
}

func validateWorkingDir(root, skillPath, name, workingDir string) []ValidationFailure {
	wd := strings.TrimSpace(workingDir)
	if wd == "" {
		return nil
	}
	clean := filepath.Clean(wd)
	if filepath.IsAbs(wd) || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return []ValidationFailure{{Path: skillPath, Message: "working_dir must stay within the skill root"}}
	}
	full := filepath.Join(root, "skills", name, clean)
	info, err := os.Stat(full)
	if err != nil {
		if os.IsNotExist(err) {
			return []ValidationFailure{{Path: skillPath, Message: "working_dir must reference an existing directory under the skill root"}}
		}
		return []ValidationFailure{{Path: skillPath, Message: fmt.Sprintf("working_dir could not be checked: %v", err)}}
	}
	if !info.IsDir() {
		return []ValidationFailure{{Path: skillPath, Message: "working_dir must reference an existing directory under the skill root"}}
	}
	return nil
}

func validateTimeout(skillPath, timeout string) []ValidationFailure {
	timeout = strings.TrimSpace(timeout)
	if timeout == "" {
		return nil
	}
	if _, err := time.ParseDuration(timeout); err != nil {
		return []ValidationFailure{{Path: skillPath, Message: fmt.Sprintf("timeout must be a valid duration: %v", err)}}
	}
	return nil
}

func validateSkillRequiredSections(skillPath string, doc domain.SkillDocument) []ValidationFailure {
	requiredSections := []string{"## What it does", "## When to use", "## How to run", "## Constraints"}
	failures := make([]ValidationFailure, 0, len(requiredSections))
	for _, section := range requiredSections {
		if !strings.Contains(doc.Body, section) {
			failures = append(failures, ValidationFailure{Path: skillPath, Message: "missing section: " + strings.TrimPrefix(section, "## ")})
		}
	}
	return failures
}
