// Command quality measures production structure using Go's parser, not regex.
package main

import (
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type function struct {
	File, Name string
	Complexity int
}

func main() {
	functions := []function{}
	sizes := map[string]int{}
	packages := map[string]int{}
	violations := []string{}
	err := filepath.WalkDir("internal", func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		data, e := os.ReadFile(path)
		if e != nil {
			return e
		}
		lines := strings.Count(string(data), "\n")
		sizes[path] = lines
		packages[filepath.Dir(path)] += lines
		if lines > 350 {
			violations = append(violations, path+": exceeds 350 lines")
		}
		set := token.NewFileSet()
		file, e := parser.ParseFile(set, path, data, 0)
		if e != nil {
			return e
		}
		for _, decl := range file.Decls {
			if fn, ok := decl.(*ast.FuncDecl); ok {
				n := complexity(fn)
				functions = append(functions, function{path, fn.Name.Name, n})
				if n > 15 {
					violations = append(violations, path+":"+fn.Name.Name+": complexity exceeds 15")
				}
			}
		}
		for _, imp := range file.Imports {
			dep, _ := strconv.Unquote(imp.Path.Value)
			if badDependency(path, dep) {
				violations = append(violations, path+": forbidden import "+dep)
			}
		}
		return nil
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	for name, n := range packages {
		if n > 1200 {
			violations = append(violations, name+": exceeds 1200 lines")
		}
	}
	report := map[string]any{"functions": functions, "file_lines": sizes, "package_lines": packages, "violations": violations, "passed": len(violations) == 0}
	data, _ := json.MarshalIndent(report, "", "  ")
	_ = os.MkdirAll("reports", 0755)
	_ = os.WriteFile("reports/structure.json", data, 0644)
	fmt.Println(string(data))
	if len(violations) > 0 {
		os.Exit(1)
	}
}

func complexity(fn *ast.FuncDecl) int {
	n := 1
	ast.Inspect(fn.Body, func(node ast.Node) bool {
		switch x := node.(type) {
		case *ast.IfStmt, *ast.ForStmt, *ast.RangeStmt:
			n++
		case *ast.CaseClause:
			if x.List != nil {
				n++
			}
		case *ast.CommClause:
			if x.Comm != nil {
				n++
			}
		case *ast.BinaryExpr:
			if x.Op == token.LAND || x.Op == token.LOR {
				n++
			}
		}
		return true
	})
	return n
}

func domainImport(dep string) bool {
	return dep == "dracma/pix/internal/domain" || strings.HasPrefix(dep, "dracma/pix/internal/domain/")
}

func applicationImport(dep string) bool {
	return domainImport(dep) || dep == "dracma/pix/internal/application" || strings.HasPrefix(dep, "dracma/pix/internal/application/")
}

func badDependency(path, dep string) bool {
	external := strings.Contains(strings.Split(dep, "/")[0], ".") || strings.HasPrefix(dep, "dracma/")
	if strings.Contains(path, "/application/") {
		return !applicationImport(dep) && external
	}
	if strings.Contains(path, "/domain/") {
		return external && !domainImport(dep)
	}
	if strings.Contains(path, "/ledger/") {
		return external && !domainImport(dep)
	}
	if strings.Contains(path, "/infrastructure/") {
		return !infrastructureImport(dep)
	}
	if strings.Contains(path, "/presentation/") {
		return !presentationImport(dep)
	}
	return false
}

func presentationImport(dep string) bool {
	if infrastructureImport(dep) {
		return true
	}
	return dep == "dracma/pix/internal/presentation" || strings.HasPrefix(dep, "dracma/pix/internal/presentation/")
}

func infrastructureImport(dep string) bool {
	if !strings.Contains(strings.Split(dep, "/")[0], ".") && !strings.HasPrefix(dep, "dracma/") {
		return true
	}
	if dep == "github.com/tigerbeetle/tigerbeetle-go" {
		return true
	}
	for _, prefix := range []string{
		"dracma/pix/internal/domain",
		"dracma/pix/internal/application",
		"dracma/pix/internal/infrastructure",
	} {
		if dep == prefix || strings.HasPrefix(dep, prefix+"/") {
			return true
		}
	}
	return false
}
