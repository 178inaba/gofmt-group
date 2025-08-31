package main

import (
	"bytes"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
)

func main() {
	root := "."
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			// Skip vendor, .git, etc.
			if d.Name() == "vendor" || d.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		if filepath.Ext(path) == ".go" {
			if err := formatFile(path); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		panic(err)
	}
}

func formatFile(filename string) error {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, filename, nil, parser.ParseComments)
	if err != nil {
		return err
	}
	// Rewrite AST
	rewriteFuncParams(file)
	// Overwrite file with formatted result
	tmp, err := os.CreateTemp("", "fmt-*.go")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if err := customFprint(tmp, fset, file); err != nil {
		tmp.Close()
		return err
	}
	tmp.Close()
	// Overwrite save
	return os.Rename(tmp.Name(), filename)
}

func rewriteFuncParams(file *ast.File) {
	ast.Inspect(file, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.FuncDecl:
			// Function declaration
			processFuncType(node.Type)
		case *ast.InterfaceType:
			// Methods in interface
			for _, method := range node.Methods.List {
				if funcType, ok := method.Type.(*ast.FuncType); ok {
					processFuncType(funcType)
				}
			}
		}
		return true
	})
}

func processFuncType(funcType *ast.FuncType) {
	// Parameters
	if funcType.Params != nil {
		funcType.Params.List = mergeSameTypeFields(funcType.Params.List)
	}
	// Return values
	if funcType.Results != nil {
		// For single unnamed return value, remove parentheses first then merge
		if len(funcType.Results.List) == 1 && len(funcType.Results.List[0].Names) == 0 {
			funcType.Results.Opening = token.NoPos
			funcType.Results.Closing = token.NoPos
		}
		funcType.Results.List = mergeSameTypeFields(funcType.Results.List)
	}
}

func mergeSameTypeFields(fields []*ast.Field) []*ast.Field {
	if len(fields) == 0 {
		return fields
	}
	var newList []*ast.Field
	for i := 0; i < len(fields); {
		field := fields[i]
		j := i + 1
		names := append([]*ast.Ident{}, field.Names...)
		for j < len(fields) {
			next := fields[j]
			// Merge if types are the same
			if typeString(field.Type) == typeString(next.Type) {
				names = append(names, next.Names...)
				j++
			} else {
				break
			}
		}
		newList = append(newList, &ast.Field{
			Names: names,
			Type:  field.Type,
		})
		i = j
	}
	return newList
}

func typeString(expr ast.Expr) string {
	var buf bytes.Buffer
	printer.Fprint(&buf, token.NewFileSet(), expr)
	return buf.String()
}

func customFprint(dst *os.File, fset *token.FileSet, file *ast.File) error {
	var buf bytes.Buffer
	cfg := &printer.Config{
		Mode:     printer.TabIndent | printer.UseSpaces,
		Tabwidth: 8,
	}
	if err := cfg.Fprint(&buf, fset, file); err != nil {
		return err
	}

	// Remove parentheses for single return value
	re := regexp.MustCompile(`\) \(([^,\(\)]+)\) \{`)
	result := re.ReplaceAllString(buf.String(), `) $1 {`)

	_, err := dst.WriteString(result)
	return err
}
