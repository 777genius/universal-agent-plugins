package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path"
	"path/filepath"
	"strings"
)

// pathPolicyOwner is the one package allowed to implement ports.PathPolicy.
const pathPolicyOwner = "install/integrationctl/adapters/pathpolicy"

// pathPolicyContract is the ports.PathPolicy method set, as method name to
// string parameter count. Every method returns a single error.
//
// depguard cannot express this rule: it matches import paths, and a permissive
// PathPolicy needs no import at all - three short methods on a local struct are
// enough to turn RequireExactPath, RequireContainedChild and their os.Lstat
// symlink rejection into no-ops for whichever service receives it.
var pathPolicyContract = map[string]int{
	"ValidateLeafID":        1,
	"RequireContainedChild": 2,
	"RequireExactPath":      2,
}

// pathPolicyImplementations maps "directory.Type" to the declaring directory
// for every type in the checkout that carries the whole contract. Test files
// count: a fake is exactly what this rule exists to reject.
func pathPolicyImplementations(root string) (map[string]string, error) {
	declared := make(map[string]map[string]struct{})
	directories := make(map[string]string)
	err := walkRepositoryFiles(root, func(rel string, file *ast.File) error {
		directory := path.Dir(rel)
		for _, declaration := range file.Decls {
			function, ok := declaration.(*ast.FuncDecl)
			if !ok || function.Recv == nil || len(function.Recv.List) != 1 {
				continue
			}
			parameters, known := pathPolicyContract[function.Name.Name]
			if !known || !acceptsStringsReturnsError(function.Type, parameters) {
				continue
			}
			key := directory + "." + receiverTypeName(function.Recv.List[0].Type)
			if declared[key] == nil {
				declared[key] = make(map[string]struct{})
			}
			declared[key][function.Name.Name] = struct{}{}
			directories[key] = directory
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	implementations := make(map[string]string)
	for key, methods := range declared {
		if len(methods) == len(pathPolicyContract) {
			implementations[key] = directories[key]
		}
	}
	return implementations, nil
}

func acceptsStringsReturnsError(signature *ast.FuncType, parameters int) bool {
	if signature.Results == nil || len(signature.Results.List) != 1 {
		return false
	}
	result, ok := signature.Results.List[0].Type.(*ast.Ident)
	if !ok || result.Name != "error" {
		return false
	}
	count := 0
	for _, field := range signature.Params.List {
		identifier, ok := field.Type.(*ast.Ident)
		if !ok || identifier.Name != "string" {
			return false
		}
		count += max(len(field.Names), 1)
	}
	return count == parameters
}

func receiverTypeName(expression ast.Expr) string {
	switch typed := expression.(type) {
	case *ast.StarExpr:
		return receiverTypeName(typed.X)
	case *ast.IndexExpr:
		return receiverTypeName(typed.X)
	case *ast.IndexListExpr:
		return receiverTypeName(typed.X)
	case *ast.Ident:
		return typed.Name
	default:
		return "?"
	}
}

// walkRepositoryFiles parses every Go file in the checkout, tests included.
func walkRepositoryFiles(root string, visit func(rel string, file *ast.File) error) error {
	fset := token.NewFileSet()
	return filepath.WalkDir(root, func(current string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if skippedDirectory(entry.Name()) {
				return fs.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(entry.Name(), ".go") {
			return nil
		}
		rel, err := filepath.Rel(root, current)
		if err != nil {
			return err
		}
		file, err := parser.ParseFile(fset, current, nil, parser.SkipObjectResolution)
		if err != nil {
			return err
		}
		return visit(filepath.ToSlash(rel), file)
	})
}

func skippedDirectory(name string) bool {
	switch name {
	case "testdata", "node_modules", "vendor", "website":
		return true
	}
	return strings.HasPrefix(name, ".") || strings.HasPrefix(name, "_")
}
