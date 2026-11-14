package main

import (
	"bytes"
	"cmp"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"slices"
	"strings"
)

// layouts maps each file with a set layout to its sections, in file order. Each key is a path within a module, where
// <name> stands for the module's name. A section lists the kinds of declaration it holds, in any order. A kind no
// section lists may sit anywhere.
var layouts = map[string][]string{
	"<name>.go":               {"Port", "request", "Key", "type const", "var", "func"},
	"service/service.go":      {"New", "type const var", "method command", "func"},
	"internal/model/model.go": {"const var", "type", "New", "command", "func method", "event", "_", "Apply Version"},
}

// layout returns src with its declarations in their layout's order, and a problem naming those it moved. It moves as
// few declarations as it can.
func layout(path string, src []byte) ([]byte, []problem, error) {
	path = filepath.ToSlash(path)
	sections, port, ok := layoutOf(path)
	if !ok {
		return src, nil, nil
	}

	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, path, src, parser.ParseComments)
	if err != nil {
		return nil, nil, err
	}

	endpoints, err := portEndpoints(port)
	if err != nil {
		return nil, nil, err
	}

	decls := declsInSections(f, sections, endpoints)
	markKept(decls)
	first := slices.IndexFunc(decls, func(p placed) bool { return !p.kept })
	if first < 0 {
		return src, nil, nil
	}

	order := reorder(decls)
	var wrong, want []string
	for i, p := range decls {
		if !p.kept {
			wrong = append(wrong, label(fset, p.decl))
		}

		want = append(want, label(fset, order[i]))
	}

	line := fset.Position(decls[first].decl.Pos()).Line
	msg := "out of order: " + strings.Join(wrong, ", ") + ". Want: " + strings.Join(want, ", ")

	return rewrite(src, spans(fset, f, src), decls, order), []problem{{line, msg, true}}, nil
}

// layoutOf returns the layout's sections for the file at path, and the path of its module's port file. It returns
// false for a file with no set layout.
func layoutOf(path string) ([]string, string, bool) {
	i := strings.LastIndex(path, "modules/")
	if i < 0 {
		return nil, "", false
	}

	module, file, _ := strings.Cut(path[i+len("modules/"):], "/")
	if file == module+".go" {
		file = "<name>.go"
	}

	sections, ok := layouts[file]

	return sections, path[:i] + "modules/" + module + "/" + module + ".go", ok
}

// placed is a declaration with its place in the layout. section indexes the layout's sections. endpoint is the
// position in Port of the field the declaration is named after, or 0. kept means the fix leaves it where it is.
type placed struct {
	decl              ast.Decl
	section, endpoint int
	kept              bool
}

// precedes reports whether p must come before q in the layout.
func (p placed) precedes(q placed) bool {
	if p.section != q.section {
		return p.section < q.section
	}

	return p.endpoint > 0 && p.endpoint < q.endpoint
}

// declsInSections returns each declaration in f whose kind one of sections lists, in file order. endpoints maps a
// lower-cased name to its position in Port.
func declsInSections(f *ast.File, sections []string, endpoints map[string]int) []placed {
	var decls []placed
	events, keys := eventTypes(f), receivers(f, "Key")
	for _, d := range f.Decls {
		k := kind(d, events, keys)
		s := slices.IndexFunc(sections, func(s string) bool { return slices.Contains(strings.Fields(s), k) })
		if s >= 0 {
			name := strings.TrimSuffix(strings.TrimSuffix(declName(d), "Cmd"), "Query")
			decls = append(decls, placed{decl: d, section: s, endpoint: endpoints[strings.ToLower(name)]})
		}
	}

	return decls
}

// markKept sets kept on each declaration that the fix leaves in place. It keeps the longest subsequence of decls whose
// sections never decrease. Within each section, it keeps the longest subsequence of declarations named after an
// endpoint, in Port's order. A declaration named after no endpoint may sit anywhere in its section.
func markKept(decls []placed) {
	named := map[int][]*placed{}
	for i, ok := range longestOrderedRun(decls, func(p placed) int { return p.section }) {
		decls[i].kept = ok
		if ok && decls[i].endpoint > 0 {
			named[decls[i].section] = append(named[decls[i].section], &decls[i])
		}
	}

	for _, run := range named {
		for j, ok := range longestOrderedRun(run, func(p *placed) int { return p.endpoint }) {
			run[j].kept = ok
		}
	}
}

// reorder returns the declarations in their layout's order. Each declaration that is not kept moves just above the
// first kept declaration it must precede.
func reorder(decls []placed) []ast.Decl {
	above := make([][]placed, len(decls)+1)
	for _, p := range slices.DeleteFunc(slices.Clone(decls), func(p placed) bool { return p.kept }) {
		j := p.slot(decls)
		above[j] = append(above[j], p)
	}

	var order []ast.Decl
	for j, moved := range above {
		slices.SortStableFunc(moved, func(a, b placed) int {
			return cmp.Or(a.section-b.section, a.endpoint-b.endpoint)
		})

		for _, p := range moved {
			order = append(order, p.decl)
		}

		if j < len(decls) && decls[j].kept {
			order = append(order, decls[j].decl)
		}
	}

	return order
}

// slot returns the index of the first kept declaration that p must precede, or len(decls) if there is none.
func (p placed) slot(decls []placed) int {
	j := slices.IndexFunc(decls, func(q placed) bool { return q.kept && p.precedes(q) })
	if j < 0 {
		return len(decls)
	}

	return j
}

// spans returns the byte offsets in src where each declaration's text starts and ends. The text starts where the
// previous declaration's text ends, so it includes the comments and blank lines above the declaration.
func spans(fset *token.FileSet, f *ast.File, src []byte) map[ast.Decl][2]int {
	text := map[ast.Decl][2]int{}
	prev := fset.Position(f.Name.End()).Offset
	for _, d := range f.Decls {
		end := fset.Position(d.End()).Offset
		end += max(0, bytes.IndexByte(src[end:], '\n'))
		text[d], prev = [2]int{prev, end}, end
	}

	return text
}

// rewrite returns src with the text of decls[k] replaced by the text of order[k], for each k. text maps each
// declaration to its span in src.
func rewrite(src []byte, text map[ast.Decl][2]int, decls []placed, order []ast.Decl) []byte {
	var out []byte
	at := 0
	for k, p := range decls {
		slot, t := text[p.decl], text[order[k]]
		out, at = append(append(out, src[at:slot[0]]...), src[t[0]:t[1]]...), slot[1]
	}

	return append(out, src[at:]...)
}

// longestOrderedRun finds the longest subsequence of items whose keys never decrease. It returns a bool for each
// item, true when the item is in that subsequence.
func longestOrderedRun[T any](items []T, key func(T) int) []bool {
	length, prev := make([]int, len(items)), make([]int, len(items))
	last := -1
	for j := range items {
		length[j], prev[j] = 1, -1
		for i := range j {
			if key(items[i]) <= key(items[j]) && length[i]+1 > length[j] {
				length[j], prev[j] = length[i]+1, i
			}
		}

		if last < 0 || length[j] > length[last] {
			last = j
		}
	}

	in := make([]bool, len(items))
	for j := last; j >= 0; j = prev[j] {
		in[j] = true
	}

	return in
}

// kind returns d's kind, as one of the words that a section in layouts lists. A type with a Key method that is not a
// request has the kind Key. An import's kind is in no layout, so it stays in place.
func kind(d ast.Decl, events, keys map[string]bool) string {
	switch d := d.(type) {
	case *ast.GenDecl:
		return genKind(d, events, keys)
	case *ast.FuncDecl:
		if d.Recv == nil {
			return funcKind(d.Name.Name)
		}

		return methodKind(d, events)
	}

	return ""
}

// genKind is kind for a type, const, var or import declaration.
func genKind(d *ast.GenDecl, events, keys map[string]bool) string {
	name, value := declName(d), valueName(d)
	switch {
	case d.Tok == token.VAR && value == "_":
		return "_"
	case events[name] || events[value]:
		return "event"
	case name == "Port":
		return "Port"
	case isRequest(name):
		return "request"
	case keys[name]:
		return "Key"
	}

	return d.Tok.String()
}

// isRequest reports whether a type called name is a command or a query.
func isRequest(name string) bool {
	return strings.HasSuffix(name, "Cmd") || strings.HasSuffix(name, "Query")
}

// ownKinds lists the names of the funcs and methods whose kind is their own name.
var ownKinds = []string{"Key", "Apply", "Version"}

// funcKind is kind for a function with the given name.
func funcKind(name string) string {
	switch {
	case slices.Contains(ownKinds, name):
		return name
	case strings.HasPrefix(name, "New"):
		return "New"
	}

	return "func"
}

// methodKind is kind for a method. A method of an event is an event, and one that returns an Event is a command.
func methodKind(d *ast.FuncDecl, events map[string]bool) string {
	switch {
	case events[recvName(d)]:
		return "event"
	case slices.Contains(ownKinds, d.Name.Name):
		return d.Name.Name
	case returnsEvent(d):
		return "command"
	}

	return "method"
}

// portEndpoints maps each Port field in the port file at path to its place in Port, counting from 1. A field's name is
// lower-cased, so it matches both an exported command and an unexported handler.
func portEndpoints(path string) (map[string]int, error) {
	f, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
	if err != nil {
		return nil, err
	}

	endpoints := map[string]int{}
	ast.Inspect(f, func(n ast.Node) bool {
		if ts, ok := n.(*ast.TypeSpec); ok && ts.Name.Name == "Port" {
			for _, field := range ts.Type.(*ast.StructType).Fields.List {
				for _, name := range field.Names {
					endpoints[strings.ToLower(name.Name)] = len(endpoints) + 1
				}
			}
		}

		return true
	})

	return endpoints, nil
}

// receivers returns the types in f that have the named method.
func receivers(f *ast.File, method string) map[string]bool {
	types := map[string]bool{}
	for _, d := range f.Decls {
		fd, ok := d.(*ast.FuncDecl)
		if !ok || fd.Recv == nil {
			continue
		}

		if fd.Name.Name == method {
			types[recvName(fd)] = true
		}
	}

	return types
}

// eventTypes returns the event types that f's codec lists, with Event and codec themselves.
func eventTypes(f *ast.File) map[string]bool {
	types := map[string]bool{"Event": true, "codec": true}
	ast.Inspect(f, func(n ast.Node) bool {
		vs, ok := n.(*ast.ValueSpec)
		if !ok || vs.Names[0].Name != "codec" {
			return true
		}

		for _, name := range codecTypes(vs) {
			types[name] = true
		}

		return false
	})

	return types
}

// codecTypes returns each type that codec's spec lists as a composite literal, such as Added{}.
func codecTypes(vs *ast.ValueSpec) []string {
	if len(vs.Values) == 0 {
		return nil
	}

	call, ok := vs.Values[0].(*ast.CallExpr)
	if !ok {
		return nil
	}

	var names []string
	for _, arg := range call.Args {
		if lit, ok := arg.(*ast.CompositeLit); ok {
			if id, ok := lit.Type.(*ast.Ident); ok {
				names = append(names, id.Name)
			}
		}
	}

	return names
}

func returnsEvent(fd *ast.FuncDecl) bool {
	r := fd.Type.Results
	if r == nil || len(r.List) == 0 {
		return false
	}

	id, ok := r.List[0].Type.(*ast.Ident)

	return ok && id.Name == "Event"
}

// label returns d's name for a problem's message. A method reads as Type.Name, and a var _ as its line number.
func label(fset *token.FileSet, d ast.Decl) string {
	switch d := d.(type) {
	case *ast.FuncDecl:
		if d.Recv != nil {
			return recvName(d) + "." + d.Name.Name
		}
	case *ast.GenDecl:
		if name := valueName(d); name == "_" {
			return fmt.Sprintf("line %d", fset.Position(d.Pos()).Line)
		} else if name != "" {
			return name
		}
	}

	return declName(d)
}

func declName(d ast.Decl) string {
	switch d := d.(type) {
	case *ast.GenDecl:
		if ts, ok := d.Specs[0].(*ast.TypeSpec); ok {
			return ts.Name.Name
		}
	case *ast.FuncDecl:
		return d.Name.Name
	}

	return ""
}

func valueName(g *ast.GenDecl) string {
	if vs, ok := g.Specs[0].(*ast.ValueSpec); ok {
		return vs.Names[0].Name
	}

	return ""
}

func recvName(fd *ast.FuncDecl) string {
	t := fd.Recv.List[0].Type
	if s, ok := t.(*ast.StarExpr); ok {
		t = s.X
	}

	if id, ok := t.(*ast.Ident); ok {
		return id.Name
	}

	return ""
}
