// Command archtest measures and enforces the architecture guardrails of the
// agentplugins install core. Run it with -update to regenerate the committed
// ClientID budget baseline; `go test ./...` re-measures and fails on growth.
package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

const (
	modulePath       = "github.com/777genius/plugin-kit-ai"
	domainImportPath = modulePath + "/install/integrationctl/agentplugins/domain"
	legacyPortsPath  = modulePath + "/install/integrationctl/ports"

	budgetFile = "testdata/client_id_budget.json"
)

// coreRoots are the three packages the refactor plan calls "the install core",
// expressed as repository-relative directories.
var coreRoots = []string{
	"install/integrationctl/agentplugins",
	"cli/internal/agentpluginscli",
	"cli/cmd/agentplugins",
}

// clientSelectorPattern is the shape the refactor plan specifies for a client
// identity reference. It is only the first filter: the pattern also matches
// domain types such as ClientID, ClientBinding and ClientSurface, which are
// plain type references and not branching on a client. Counting those would
// make the budget grow whenever a signature gains a map[domain.ClientID], and
// would put the plan's own "zero client branching" goal out of reach, since
// usecase and ports name that type by necessity. The second filter therefore
// keeps only names the domain package declares as ClientID constants.
var clientSelectorPattern = regexp.MustCompile(`^Client[A-Z][A-Za-z0-9]*$`)

// budget is the committed baseline: selector count per repository-relative
// package directory. Packages at zero are omitted.
type budget struct {
	Packages map[string]int `json:"packages"`
}

// repoRoot walks up from the working directory to the checkout that owns the
// go.work file. Tests run from their package directory, the tool from anywhere.
func repoRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.work")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("no go.work found above the working directory")
		}
		dir = parent
	}
}

// clientIDBudget counts client-identity selectors in every non-test file of the
// core, grouped by package and skipping the allow-listed directories.
func clientIDBudget(root string) (map[string]int, error) {
	identities, err := clientIdentityConstants(root)
	if err != nil {
		return nil, err
	}
	counts := make(map[string]int)
	err = walkCoreFiles(root, func(rel string, file *ast.File) error {
		if budgetAllowListed(rel) {
			return nil
		}
		count := countClientSelectors(file, identities)
		if count == 0 {
			return nil
		}
		counts[path.Dir(rel)] += count
		return nil
	})
	if err != nil {
		return nil, err
	}
	return counts, nil
}

// clientIdentityConstants reads the declarative registry to learn which
// Client* names are identity constants rather than types or functions. Reading
// the declaration keeps the metric in step with a newly added client without a
// second list to maintain.
func clientIdentityConstants(root string) (map[string]struct{}, error) {
	identities := make(map[string]struct{})
	domainDir := filepath.Join(root, filepath.FromSlash("install/integrationctl/agentplugins/domain"))
	entries, err := os.ReadDir(domainDir)
	if err != nil {
		return nil, err
	}
	fset := token.NewFileSet()
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		file, err := parser.ParseFile(fset, filepath.Join(domainDir, entry.Name()), nil, parser.SkipObjectResolution)
		if err != nil {
			return nil, err
		}
		collectClientIdentities(file, identities)
	}
	if len(identities) == 0 {
		return nil, fmt.Errorf("no ClientID constants found in %s", domainDir)
	}
	return identities, nil
}

func collectClientIdentities(file *ast.File, identities map[string]struct{}) {
	for _, declaration := range file.Decls {
		general, ok := declaration.(*ast.GenDecl)
		if !ok || general.Tok != token.CONST {
			continue
		}
		// Inside one const block a spec without a type repeats the previous one.
		declared := ""
		for _, spec := range general.Specs {
			value, ok := spec.(*ast.ValueSpec)
			if !ok {
				continue
			}
			if identifier, ok := value.Type.(*ast.Ident); ok {
				declared = identifier.Name
			} else if value.Type != nil {
				declared = ""
			} else if len(value.Values) > 0 {
				// A spec inherits the previous type only when its expression
				// list is empty. Its own untyped value ends the inheritance.
				declared = ""
			}
			if declared != "ClientID" {
				continue
			}
			for _, name := range value.Names {
				if clientSelectorPattern.MatchString(name.Name) {
					identities[name.Name] = struct{}{}
				}
			}
		}
	}
}

// budgetAllowListed names the places where client-specific code is the point:
// the declarative registry and, once the refactor creates them, the per-client
// adapter packages.
func budgetAllowListed(rel string) bool {
	if rel == "install/integrationctl/agentplugins/domain/clients.go" {
		return true
	}
	const clientsRoot = "install/integrationctl/agentplugins/clients/"
	if !strings.HasPrefix(rel, clientsRoot) {
		return false
	}
	leaf, _, found := strings.Cut(strings.TrimPrefix(rel, clientsRoot), "/")
	if !found {
		return false
	}
	if leaf == "all" {
		return true
	}
	for _, id := range domain.SupportedClientIDs() {
		if leaf == string(id) {
			return true
		}
	}
	return false
}

func countClientSelectors(file *ast.File, identities map[string]struct{}) int {
	local := importName(file, domainImportPath)
	if local == "" {
		return 0
	}
	count := 0
	ast.Inspect(file, func(node ast.Node) bool {
		selector, ok := node.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		// Without type information a local identifier that shadows the package
		// name would be miscounted. Nothing in the core does that today, and
		// the alternative is a full type-checking dependency.
		qualifier, ok := selector.X.(*ast.Ident)
		if !ok || qualifier.Name != local {
			return true
		}
		if _, identity := identities[selector.Sel.Name]; identity {
			count++
		}
		return true
	})
	return count
}

// walkCoreFiles parses every non-test Go file under the core roots. Files are
// visited in a deterministic order so generated output never reorders.
func walkCoreFiles(root string, visit func(rel string, file *ast.File) error) error {
	fset := token.NewFileSet()
	for _, core := range coreRoots {
		base := filepath.Join(root, filepath.FromSlash(core))
		err := filepath.WalkDir(base, func(current string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.IsDir() {
				if entry.Name() == "testdata" || strings.HasPrefix(entry.Name(), "_") {
					return fs.SkipDir
				}
				return nil
			}
			if !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
				return nil
			}
			rel, err := filepath.Rel(root, current)
			if err != nil {
				return err
			}
			// Build tags are ignored on purpose: a platform-specific file still
			// counts, otherwise the budget would depend on the measuring host.
			file, err := parser.ParseFile(fset, current, nil, parser.SkipObjectResolution)
			if err != nil {
				return err
			}
			return visit(filepath.ToSlash(rel), file)
		})
		if err != nil {
			return err
		}
	}
	return nil
}

// importName returns the identifier that qualifies references to want in this
// file, honoring an explicit alias, or "" when the file does not import it.
func importName(file *ast.File, want string) string {
	for _, spec := range file.Imports {
		value, err := strconv.Unquote(spec.Path.Value)
		if err != nil || value != want {
			continue
		}
		if spec.Name != nil {
			if spec.Name.Name == "_" || spec.Name.Name == "." {
				return ""
			}
			return spec.Name.Name
		}
		return path.Base(value)
	}
	return ""
}
