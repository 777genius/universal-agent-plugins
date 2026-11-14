package runtimecheck

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func inspectNode(root, entrypoint string) NodeShape {
	shape := NodeShape{
		Manager:         detectNodeManager(root),
		LauncherTarget:  detectNodeRuntimeTarget(root, entrypoint),
		OutputDir:       "dist",
		PackageJSONPath: "package.json",
	}
	loadNodePackageJSON(root, &shape)
	loadNodeTSConfig(root, &shape)

	shape.ManagerBinary = string(shape.Manager)
	shape.ManagerAvailable = lookupBinary(shape.ManagerBinary)
	shape.UsesBuiltOutput = isBuiltOutputTarget(shape.LauncherTarget, shape.OutputDir)
	shape.RuntimeTarget = shape.LauncherTarget
	shape.RuntimeTargetOK = fileExists(filepath.Join(root, filepath.FromSlash(shape.RuntimeTarget)))
	shape.Installed = nodeInstallStatePresent(root)
	applyNodeStructuralIssue(&shape)
	return shape
}

func loadNodePackageJSON(root string, shape *NodeShape) {
	body, err := os.ReadFile(filepath.Join(root, "package.json"))
	if err != nil {
		return
	}
	shape.PackageJSON = true
	var pkg packageJSON
	if json.Unmarshal(body, &pkg) == nil {
		shape.BuildScript = strings.TrimSpace(pkg.Scripts["build"])
		shape.PackageManager = strings.TrimSpace(pkg.PackageManager)
	}
}

func loadNodeTSConfig(root string, shape *NodeShape) {
	if !fileExists(filepath.Join(root, "tsconfig.json")) {
		return
	}
	shape.TSConfig = true
	shape.TSConfigPath = "tsconfig.json"
	if outDir := parseTSOutDir(root); outDir != "" {
		shape.OutputDir = outDir
	}
}

func applyNodeStructuralIssue(shape *NodeShape) {
	if !shape.PackageJSON {
		shape.StructuralIssue = "package.json is missing for node runtime"
		return
	}
	if shape.UsesBuiltOutput {
		if !shape.TSConfig {
			shape.StructuralIssue = fmt.Sprintf("launcher target %s points to built output but tsconfig.json is missing", shape.LauncherTarget)
			return
		}
		if shape.BuildScript == "" {
			shape.StructuralIssue = "TypeScript lane is missing package.json scripts.build"
			return
		}
		if !strings.HasPrefix(shape.LauncherTarget, shape.OutputDir+"/") {
			shape.StructuralIssue = fmt.Sprintf("launcher target %s is outside tsconfig outDir %s", shape.LauncherTarget, shape.OutputDir)
			return
		}
		shape.IsTypeScript = true
		return
	}
	if shape.TSConfig && shape.BuildScript != "" && strings.HasSuffix(shape.LauncherTarget, ".js") && !strings.HasPrefix(shape.LauncherTarget, shape.OutputDir+"/") {
		shape.StructuralIssue = fmt.Sprintf("launcher target %s is outside tsconfig outDir %s", shape.LauncherTarget, shape.OutputDir)
	}
}
