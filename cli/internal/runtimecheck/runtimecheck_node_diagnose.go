package runtimecheck

import "fmt"

func diagnoseNode(project Project, nextValidate string) Diagnosis {
	shape := project.Node
	if shape.StructuralIssue != "" {
		return Diagnosis{
			Status: StatusBlocked,
			Reason: shape.StructuralIssue,
			Next:   []string{"plugin-kit-ai bootstrap .", nextValidate},
		}
	}
	if !shape.ManagerAvailable {
		return Diagnosis{
			Status: StatusBlocked,
			Reason: fmt.Sprintf("%s not found in PATH", shape.ManagerBinary),
			Next:   []string{"plugin-kit-ai bootstrap .", nextValidate},
		}
	}
	if !shape.Installed {
		return Diagnosis{
			Status: StatusNeedsBootstrap,
			Reason: fmt.Sprintf("%s install state is missing", shape.ManagerDisplay()),
			Next:   []string{"plugin-kit-ai bootstrap .", nextValidate},
		}
	}
	if shape.IsTypeScript && !shape.RuntimeTargetOK {
		return Diagnosis{
			Status: StatusNeedsBuild,
			Reason: fmt.Sprintf("built output %s is missing", shape.RuntimeTarget),
			Next:   []string{"plugin-kit-ai bootstrap .", nextValidate},
		}
	}
	if !shape.RuntimeTargetOK {
		return Diagnosis{
			Status: StatusBlocked,
			Reason: fmt.Sprintf("runtime target %s is missing", shape.RuntimeTarget),
			Next:   []string{nextValidate},
		}
	}
	return Diagnosis{
		Status: StatusReady,
		Reason: fmt.Sprintf("Node runtime is ready via %s", shape.ManagerDisplay()),
		Next:   []string{nextValidate},
	}
}

func (n NodeShape) ManagerDisplay() string {
	if n.Manager == "" {
		return "npm"
	}
	return string(n.Manager)
}

func (n NodeShape) BuildCommandString() string {
	switch n.Manager {
	case NodeManagerPNPM:
		return "pnpm run build"
	case NodeManagerYarn:
		return "yarn build"
	case NodeManagerBun:
		return "bun run build"
	default:
		return "npm run build"
	}
}
