package app

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/777genius/plugin-kit-ai/cli/internal/runtimecheck"
)

func bootstrapNode(ctx context.Context, project runtimecheck.Project) ([]string, error) {
	root := project.Root
	shape := project.Node
	lines := []string{fmt.Sprintf("Detected Node manager: %s", shape.ManagerDisplay())}
	if !shape.ManagerAvailable {
		return nil, fmt.Errorf("bootstrap failed: %s not found in PATH", shape.ManagerBinary)
	}
	switch shape.Manager {
	case runtimecheck.NodeManagerPNPM:
		if err := runBootstrapCommand(ctx, root, "pnpm", "install", "--frozen-lockfile"); err != nil {
			return nil, err
		}
		lines = append(lines, "Ran: pnpm install --frozen-lockfile")
	case runtimecheck.NodeManagerYarn:
		if runtimecheck.YarnBerry(root, shape.PackageManager) {
			if err := runBootstrapCommand(ctx, root, "yarn", "install", "--immutable"); err != nil {
				return nil, err
			}
			lines = append(lines, "Ran: yarn install --immutable")
		} else {
			if err := runBootstrapCommand(ctx, root, "yarn", "install", "--frozen-lockfile"); err != nil {
				return nil, err
			}
			lines = append(lines, "Ran: yarn install --frozen-lockfile")
		}
	case runtimecheck.NodeManagerBun:
		if err := runBootstrapCommand(ctx, root, "bun", "install"); err != nil {
			return nil, err
		}
		lines = append(lines, "Ran: bun install")
	default:
		if fileExists(filepath.Join(root, "package-lock.json")) || fileExists(filepath.Join(root, "npm-shrinkwrap.json")) {
			if err := runBootstrapCommand(ctx, root, "npm", "ci", "--no-audit", "--no-fund"); err != nil {
				return nil, err
			}
			lines = append(lines, "Ran: npm ci --no-audit --no-fund")
		} else {
			if err := runBootstrapCommand(ctx, root, "npm", "install", "--no-audit", "--no-fund"); err != nil {
				return nil, err
			}
			lines = append(lines, "Ran: npm install --no-audit --no-fund")
		}
	}
	if shape.IsTypeScript {
		if strings.TrimSpace(shape.BuildScript) == "" {
			return nil, fmt.Errorf("bootstrap failed: TypeScript lane detected but package.json is missing a build script")
		}
		if err := runBootstrapCommand(ctx, root, shape.ManagerBinary, buildCommandArgs(shape.Manager)...); err != nil {
			return nil, err
		}
		lines = append(lines, "Ran: "+shape.BuildCommandString())
	}
	return lines, nil
}

func buildCommandArgs(manager runtimecheck.NodeManager) []string {
	switch manager {
	case runtimecheck.NodeManagerYarn:
		return []string{"build"}
	case runtimecheck.NodeManagerBun, runtimecheck.NodeManagerPNPM:
		return []string{"run", "build"}
	default:
		return []string{"run", "build"}
	}
}
