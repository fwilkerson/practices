package repotest

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/fwilkerson/practices/claim"
)

// Names fails t for each code span in docs that names nothing the repository's Go code declares. Each of docs is a
// path or a glob of Markdown files, such as "README.md" or ".claude/agents/*.md". Names skips a lower-case word, a
// name in capitals and a Markdown file's name. It also skips a name from a package outside the repository, such as
// http.Get. A doc names a method or a field as Type.Name, and may put its package first.
func Names(t testing.TB, docs ...string) {
	t.Helper()
	problems, err := unknownNames(".", docs)
	claim.Must(t, err)

	for _, p := range problems {
		t.Error(p)
	}
}

// unknownNames returns a problem for each code span, in the docs under root, that names nothing root's Go files
// declare. A pattern of docs that matches no file is an error, so a misspelled path never passes.
func unknownNames(root string, docs []string) ([]string, error) {
	known, err := declaredUnder(root)
	if err != nil {
		return nil, err
	}

	var problems []string
	for _, pattern := range docs {
		found, _ := filepath.Glob(filepath.Join(root, pattern))
		if len(found) == 0 {
			return nil, fmt.Errorf("%s matches no file under %s", pattern, root)
		}

		for _, doc := range found {
			unknown, err := known.unknownIn(root, doc)
			if err != nil {
				return nil, err
			}

			problems = append(problems, unknown...)
		}
	}

	return problems, nil
}

// names is what a repository's Go code declares, and the packages it declares them in.
type names struct {
	declared, packages map[string]bool
}

// declaredUnder reads the names that the Go files under root declare. It skips testdata, and each directory whose
// name starts with a dot.
func declaredUnder(root string) (names, error) {
	k := names{declared: map[string]bool{}, packages: map[string]bool{}}
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		switch {
		case err != nil:
			return err
		case skipped(root, path, d):
			return filepath.SkipDir
		case !strings.HasSuffix(path, ".go"):
			return nil
		}

		return k.read(path)
	})

	return k, err
}

// skipped reports whether d, at path, is a directory below root that declaredUnder leaves out.
func skipped(root, path string, d fs.DirEntry) bool {
	if !d.IsDir() || path == root {
		return false
	}

	return strings.HasPrefix(d.Name(), ".") || d.Name() == "testdata"
}

// read adds the names the Go file at path declares, alone and qualified by its package. A file of package x_test is
// read as package x.
func (k names) read(path string) error {
	f, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.SkipObjectResolution)
	if err != nil {
		return err
	}

	pkg := strings.TrimSuffix(f.Name.Name, "_test")
	k.packages[pkg] = true
	for _, name := range declarations(f) {
		k.declared[name] = true
		k.declared[pkg+"."+name] = true
	}

	return nil
}

// codeSpan is a code span that holds a Go name, qualified or not.
var codeSpan = regexp.MustCompile("`([A-Za-z]\\w*(?:\\.[A-Za-z]\\w*)*)`")

// unknownIn returns a problem for each code span of the doc at path that names nothing k holds. Each problem names
// the doc by its path under root.
func (k names) unknownIn(root, path string) ([]string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	doc, err := filepath.Rel(root, path)
	if err != nil {
		return nil, err
	}

	var problems []string
	for i, line := range strings.Split(string(b), "\n") {
		for _, m := range codeSpan.FindAllStringSubmatch(line, -1) {
			if !k.allows(m[1]) {
				problems = append(problems, fmt.Sprintf(
					"%s:%d: `%s` names nothing in this repository. Fix it, or write it in words.", doc, i+1, m[1]))
			}
		}
	}

	return problems, nil
}

// allows reports whether a doc may name name: the repository declares it, or Names skips it.
func (k names) allows(name string) bool {
	qualifier, _, qualified := strings.Cut(name, ".")
	switch {
	case name == strings.ToLower(name), name == strings.ToUpper(name), strings.HasSuffix(name, ".md"):
		return true
	case qualified && k.outside(qualifier):
		return true
	}

	return k.declared[name]
}

// outside reports whether qualifier looks like a package the repository does not declare, such as http. It takes
// every lower-case qualifier for a package's name.
func (k names) outside(qualifier string) bool {
	return qualifier == strings.ToLower(qualifier) && !k.packages[qualifier]
}

// declarations lists the names a file declares at its top level, with each method and field also as Type.Name.
func declarations(f *ast.File) []string {
	var names []string
	for _, decl := range f.Decls {
		switch d := decl.(type) {
		case *ast.FuncDecl:
			names = append(names, funcNames(d)...)
		case *ast.GenDecl:
			names = append(names, genNames(d)...)
		}
	}

	return names
}

// funcNames lists a func's name, and a method's also as Type.Name.
func funcNames(d *ast.FuncDecl) []string {
	if d.Recv == nil {
		return []string{d.Name.Name}
	}

	recv, _, _ := strings.Cut(strings.TrimPrefix(types.ExprString(d.Recv.List[0].Type), "*"), "[")
	return []string{d.Name.Name, recv + "." + d.Name.Name}
}

// genNames lists the types, constants and variables d declares, with each type's fields and methods.
func genNames(d *ast.GenDecl) []string {
	var names []string
	for _, spec := range d.Specs {
		switch s := spec.(type) {
		case *ast.TypeSpec:
			names = append(names, typeNames(s)...)
		case *ast.ValueSpec:
			for _, n := range s.Names {
				names = append(names, n.Name)
			}
		}
	}

	return names
}

// typeNames lists a type's name, and each of its fields or methods as Type.Name.
func typeNames(s *ast.TypeSpec) []string {
	names := []string{s.Name.Name}
	switch typ := s.Type.(type) {
	case *ast.StructType:
		return append(names, members(s.Name.Name, typ.Fields)...)
	case *ast.InterfaceType:
		return append(names, members(s.Name.Name, typ.Methods)...)
	}

	return names
}

// members lists each name of fields, alone and as typ.Name.
func members(typ string, fields *ast.FieldList) []string {
	var names []string
	for _, field := range fields.List {
		for _, n := range field.Names {
			names = append(names, n.Name, typ+"."+n.Name)
		}
	}

	return names
}
