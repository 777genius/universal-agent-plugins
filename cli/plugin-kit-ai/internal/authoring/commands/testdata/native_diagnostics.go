//go:build ignore

// This CI-only overlay observes the actual Windows test/entrypoint invocation.
// It never edits checked-in production sources. Instrumented binaries are
// diagnostic artifacts, not uninstrumented native release proof.
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
)

func must(err error) {
	if err != nil {
		panic(err)
	}
}
func literal(s string) ast.Expr { return &ast.BasicLit{Kind: token.STRING, Value: strconv.Quote(s)} }
func errorResult(t *ast.FuncType) bool {
	if t.Results == nil {
		return false
	}
	r := t.Results.List
	id, ok := r[len(r)-1].Type.(*ast.Ident)
	return ok && id.Name == "error"
}
func main() {
	if len(os.Args) != 3 {
		panic("usage: native_diagnostics.go checkout evidence-directory")
	}
	root, err := filepath.Abs(os.Args[1])
	must(err)
	out, err := filepath.Abs(os.Args[2])
	must(err)
	must(os.MkdirAll(out, 0700))
	replacements := map[string]string{}
	packages := map[string]string{
		"install/integrationctl/agentplugins/adapters/packageview": "packageview",
		"cli/plugin-kit-ai/internal/authoring/scaffold":            "scaffold",
	}
	files := map[string][]string{
		"packageview": {"source_windows.go", "scratch_windows.go"},
		"scaffold":    {"apply.go", "rename_windows.go", "stage_windows.go"},
	}
	for dir, pkg := range packages {
		for _, base := range files[pkg] {
			path := filepath.Join(root, dir, base)
			fs := token.NewFileSet()
			f, err := parser.ParseFile(fs, path, nil, parser.ParseComments)
			must(err)
			// Wrap returned errors without replacing or unwrapping them. The site refers
			// to the original source line, and never contains an input pathname/value.
			arity := 0
			function := ""
			changes := 0
			var visit func(ast.Node) bool
			visit = func(n ast.Node) bool {
				switch n := n.(type) {
				case *ast.FuncDecl:
					function = n.Name.Name
					if errorResult(n.Type) {
						previous := arity
						arity = n.Type.Results.NumFields()
						ast.Inspect(n.Body, visit)
						arity = previous
					}
					return false
				case *ast.FuncLit:
					if errorResult(n.Type) {
						previous := arity
						arity = n.Type.Results.NumFields()
						ast.Inspect(n.Body, visit)
						arity = previous
					}
					return false
				case *ast.ReturnStmt:
					if len(n.Results) != arity {
						return false
					}
					i := len(n.Results) - 1
					if id, ok := n.Results[i].(*ast.Ident); ok && id.Name == "nil" {
						return false
					}
					site := fmt.Sprintf("%s:%d", base, fs.Position(n.Pos()).Line)
					ast.Inspect(n.Results[i], visit)
					var observations []ast.Expr
					if function == "remember" {
						if c, ok := n.Results[i].(*ast.CallExpr); ok {
							if id, ok := c.Fun.(*ast.Ident); ok && id.Name == "fail" && len(c.Args) == 1 {
								if code, ok := c.Args[0].(*ast.BasicLit); ok && code.Value == strconv.Quote("source_changed") {
									pairs := []string{"[]any{meta, after}", "[]any{meta, locked}"}
									observation, err := parser.ParseExpr(pairs[changes])
									must(err)
									observations = append(observations, observation)
									changes++
								}
							}
						}
					}
					n.Results[i] = &ast.CallExpr{Fun: ast.NewIdent("uapDiagnosticError"), Args: append([]ast.Expr{n.Results[i], literal(site)}, observations...)}
					return false
				case *ast.CallExpr:
					if s, ok := n.Fun.(*ast.SelectorExpr); ok {
						if x, ok := s.X.(*ast.Ident); ok && x.Name == "windows" && s.Sel.Name == "NtCreateFile" {
							// Capture the raw returned NTSTATUS and IOSB before winReopen maps it.
							site := literal(fmt.Sprintf("%s:%d/NtCreateFile", base, fs.Position(n.Pos()).Line))
							n.Fun = ast.NewIdent("uapDiagnosticNtCreateFile")
							n.Args = append(n.Args, site)
						} else if x, ok := s.X.(*ast.Ident); ok && x.Name == "windows" {
							site := literal(fmt.Sprintf("%s:%d/%s", base, fs.Position(n.Pos()).Line, s.Sel.Name))
							switch s.Sel.Name {
							case "NtSetInformationFile":
								n.Fun = ast.NewIdent("uapDiagnosticNtSetInformationFile")
								n.Args = append(n.Args, site)
							case "GetFileType":
								n.Fun = ast.NewIdent("uapDiagnosticGetFileType")
								n.Args = append(n.Args, site)
							case "GetFileInformationByHandle", "GetFileInformationByHandleEx", "GetVolumeInformationByHandle":
								original := *n
								n.Fun = ast.NewIdent("uapDiagnosticError")
								n.Args = []ast.Expr{&original, site}
								return false
							}
						}
					}
				}
				return true
			}
			ast.Inspect(f, visit)
			// Named result observations run after existing cleanup defers.
			for _, decl := range f.Decls {
				fn, ok := decl.(*ast.FuncDecl)
				if !ok || (fn.Name.Name != "apply" && fn.Name.Name != "openSource") {
					continue
				}
				deferred, err := parser.ParseExpr("func() { uapDiagnosticError(err, " + strconv.Quote(fn.Name.Name+".final") + ") }")
				must(err)
				fn.Body.List = append([]ast.Stmt{&ast.DeferStmt{Call: &ast.CallExpr{Fun: deferred}}}, fn.Body.List...)
			}
			var b bytes.Buffer
			must(format.Node(&b, fs, f))
			generated := filepath.Join(out, pkg+"-"+base)
			must(os.WriteFile(generated, b.Bytes(), 0600))
			replacements[path] = generated
		}
		safeCode := ""
		if pkg == "packageview" {
			safeCode = `var safe *Error; if errors.As(err, &safe) { row["code"] = safe.Code }`
		}
		helper := fmt.Sprintf(helperSource, pkg, out, pkg, safeCode)
		formatted, err := format.Source([]byte(helper))
		must(err)
		generated := filepath.Join(out, pkg+"-diagnostic_windows.go")
		must(os.WriteFile(generated, formatted, 0600))
		replacements[filepath.Join(root, dir, "uap_diagnostic_windows.go")] = generated
	}
	b, err := json.MarshalIndent(map[string]any{"Replace": replacements}, "", "  ")
	must(err)
	must(os.WriteFile(filepath.Join(out, "overlay.json"), b, 0600))
	must(os.WriteFile(filepath.Join(out, "DIAGNOSTIC_ONLY.txt"), []byte("CI-only error observations in the actual invocation. Overlay sources are preserved here. Instrumented binaries are not uninstrumented native release proof. No retries or public error changes.\n"), 0600))
}

const helperSource = `//go:build windows
package %s
import (
 "encoding/json"
 "errors"
 "fmt"
 "os"
 "path/filepath"
 "runtime"
 "strconv"
 "strings"
 "sync"
 "syscall"
 "time"
 "golang.org/x/sys/windows"
)
var uapDiagnosticMu sync.Mutex
// The destination is baked into CI-only files: subprocess profile isolation
// cannot drop it, and no user environment can enable it in a normal build.
const uapDiagnosticDirectory = %q
const uapDiagnosticPackage = %q
func uapDiagnosticError(err error, stage string, observations ...any) error {
 if err != nil { uapDiagnosticRecord(err, stage, nil, observations...) }; return err
}
func uapDiagnosticRecord(err error, stage string, native map[string]uint64, observations ...any) {
 if err == nil { return }
 // Only the numeric goroutine identifier is retained; no stack/argument text.
 var stack [64]byte
 fields := strings.Fields(string(stack[:runtime.Stack(stack[:], false)]))
 gid, _ := strconv.ParseUint(fields[1], 10, 64)
 row := map[string]any{"time_unix_nano": time.Now().UnixNano(), "pid": os.Getpid(), "goroutine": gid, "stage": stage, "type": fmt.Sprintf("%%T", err), "native": native, "observations": observations}
 var status windows.NTStatus; var errno syscall.Errno
 if errors.As(err, &status) { row["ntstatus"] = uint32(status); row["win32"] = uint32(status.Errno()) }
 if errors.As(err, &errno) { row["errno"] = uint32(errno) }
 %s
 // No Error(), pathname, SID, content, environment, or handle value is logged.
 uapDiagnosticMu.Lock(); defer uapDiagnosticMu.Unlock()
 f, e := os.OpenFile(filepath.Join(uapDiagnosticDirectory, fmt.Sprintf("%%s-%%d.jsonl", uapDiagnosticPackage, os.Getpid())), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
 if e != nil { panic("CI diagnostic sink unavailable") }
 e = json.NewEncoder(f).Encode(row); closeErr := f.Close()
 if e != nil || closeErr != nil { panic("CI diagnostic sink write failed") }
}
func uapDiagnosticNtSetInformationFile(h windows.Handle, iosb *windows.IO_STATUS_BLOCK, data *byte, size, class uint32, stage string) error {
 err := windows.NtSetInformationFile(h, iosb, data, size, class)
 if err != nil { uapDiagnosticRecord(err, stage, map[string]uint64{"class": uint64(class), "iosb_status": uint64(uint32(iosb.Status)), "iosb_information": uint64(iosb.Information)}) }
 return err
}
func uapDiagnosticGetFileType(h windows.Handle, stage string) (uint32, error) {
 kind, err := windows.GetFileType(h); uapDiagnosticRecord(err, stage, nil); return kind, err
}
func uapDiagnosticNtCreateFile(h *windows.Handle, access uint32, oa *windows.OBJECT_ATTRIBUTES, iosb *windows.IO_STATUS_BLOCK, allocation *int64, attributes, share, disposition, options uint32, ea uintptr, eaLength uint32, stage string) error {
 err := windows.NtCreateFile(h, access, oa, iosb, allocation, attributes, share, disposition, options, ea, eaLength)
 if err != nil { uapDiagnosticRecord(err, stage, map[string]uint64{"access": uint64(access), "share": uint64(share), "options": uint64(options), "disposition": uint64(disposition), "iosb_status": uint64(uint32(iosb.Status)), "iosb_information": uint64(iosb.Information)}) }
 return err
}
`
