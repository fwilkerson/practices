// Command health scores each Go file under a directory from 10 to 1, by six rules a tool can count:
//
//	$ go tool health .
//	order/order.go:11: Total takes 5 parameters, over 4
//	order/order.go:16: Total has a condition with 2 of && and ||
//	 8  order/order.go
//
// It prints each failure, then the score of each file under 10. It exits 1 when a file scores under 10.
//
//	rule          a function fails it when
//	complexity    its cognitive complexity is over 15
//	length        it holds over 80 lines of code
//	condition     one condition of an if, a for or a case holds two or more of && and ||
//	arguments     it takes more than 4 parameters
//	duplicate     its body of 10 lines or more repeats another's, but for the names and literals
//	pass-through  it only hands its parameters on to a function of its package, or a method of its receiver
//
// A file starts at 10 and loses a point for each failure, down to 1. A count of at least twice the least that fails
// loses two, such as 10 parameters. A duplicate or a pass-through costs one point.
//
// The command reads the directory it is given, or the working directory. It scores test files too. It skips the
// directories that ./... skips: testdata, vendor, and each name that starts with . or _. A duplicate is reported at
// the later body, with files read in the order of their paths. A line of code holds more than a comment.
//
// Cognitive complexity is SonarSource's measure: each break in the flow costs 1, plus 1 for each level it is nested.
// A reader takes a switch one case at a time. So a switch costs 1 plus its costliest case, where SonarSource adds up
// every case. Of a switch's lines, only its longest case counts.
package main

import (
	"bytes"
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/scanner"
	"go/token"
	"hash/fnv"
	"io/fs"
	"iter"
	"log"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

const (
	maxComplexity = 15
	maxLines      = 80
	maxOperators  = 1
	maxArguments  = 4
	minDuplicate  = 10
)

// failure is one rule that a function fails at a line. cost is the points it takes from its file.
type failure struct {
	path string
	line int
	msg  string
	cost int
}

func main() {
	flag.Parse()
	root := "."
	if flag.NArg() > 0 {
		root = flag.Arg(0)
	}

	failures, err := check(root)
	if err != nil {
		log.Fatal(err)
	}

	lost := map[string]int{}
	for _, f := range failures {
		fmt.Printf("%s:%d: %s\n", f.path, f.line, f.msg)
		lost[f.path] += f.cost
	}

	for _, path := range slices.Sorted(maps.Keys(lost)) {
		fmt.Printf("%2d  %s\n", max(10-lost[path], 1), path)
	}

	if len(failures) > 0 {
		os.Exit(1)
	}
}

// check returns each failure in the Go files under root, file by file in the order of their paths.
func check(root string) ([]failure, error) {
	var failures []failure
	seen := map[uint64]string{}
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		switch {
		case err != nil:
			return err
		case d.IsDir():
			return skipped(root, path)
		case !strings.HasSuffix(path, ".go"):
			return nil
		}

		src, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		found, err := checkFile(path, src, seen)
		failures = append(failures, found...)

		return err
	})

	return failures, err
}

// skipped returns filepath.SkipDir for a directory at path that ./... leaves out. It never leaves out root.
func skipped(root, path string) error {
	dir := filepath.Base(path)
	leftOut := strings.HasPrefix(dir, ".") || strings.HasPrefix(dir, "_") || dir == "testdata" || dir == "vendor"
	if leftOut && path != root {
		return filepath.SkipDir
	}

	return nil
}

// checkFile returns each failure in the file at path, which holds src. seen maps the shape of each body checked so
// far to where it is, and checkFile adds this file's.
func checkFile(path string, src []byte, seen map[uint64]string) ([]failure, error) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, path, src, parser.SkipObjectResolution)
	if err != nil {
		return nil, err
	}

	var failures []failure
	file := source{fset: fset, path: path, src: src, lines: bytes.Split(src, []byte("\n"))}
	for _, d := range f.Decls {
		if fn, ok := d.(*ast.FuncDecl); ok && fn.Body != nil {
			failures = append(failures, file.failures(fn, seen)...)
		}
	}

	return failures, nil
}

// source is one parsed file. lines holds each line of src.
type source struct {
	fset  *token.FileSet
	path  string
	src   []byte
	lines [][]byte
}

// failures returns a failure for each rule that fn fails. It records fn's body in seen.
func (s source) failures(fn *ast.FuncDecl, seen map[uint64]string) []failure {
	var failures []failure
	fail := func(at token.Pos, n, limit int, format string, args ...any) {
		if n > limit {
			cost := 1 + min(n/(2*limit+2), 1)
			msg := fn.Name.Name + " " + fmt.Sprintf(format, args...)
			failures = append(failures, failure{s.path, s.fset.Position(at).Line, msg, cost})
		}
	}

	n := stmts(fn.Body.List, 0)
	fail(fn.Pos(), n, maxComplexity, "has a cognitive complexity of %d, over %d", n, maxComplexity)
	n = s.code(fn) - s.folded(fn.Body)
	fail(fn.Pos(), n, maxLines, "holds %d lines of code, over %d", n, maxLines)
	n = fn.Type.Params.NumFields()
	fail(fn.Pos(), n, maxArguments, "takes %d parameters, over %d", n, maxArguments)
	for cond := range conditions(fn.Body) {
		n := operators(cond)
		fail(cond.Pos(), n, maxOperators, "has a condition with %d of && and ||", n)
	}

	if callee, ok := passesThrough(fn); ok {
		fail(fn.Pos(), 1, 0, "only hands its parameters on to %s", callee)
	}

	if first, ok := s.repeated(fn, seen); ok {
		fail(fn.Pos(), 1, 0, "repeats the body of %s", first)
	}

	return failures
}

// code counts the lines of n that hold code. A blank line holds none, and neither does a line that starts with //.
func (s source) code(n ast.Node) (count int) {
	for _, line := range s.lines[s.fset.Position(n.Pos()).Line-1 : s.fset.Position(n.End()).Line] {
		if text := bytes.TrimSpace(line); len(text) > 0 && !bytes.HasPrefix(text, []byte("//")) {
			count++
		}
	}

	return count
}

// folded counts the lines of code under root that a reader skips. For each outermost switch, that is every case
// but its longest.
func (s source) folded(root ast.Node) (count int) {
	ast.Inspect(root, func(n ast.Node) bool {
		clauses, ok := cases(n)
		if !ok {
			return true
		}

		all, longest := 0, 0
		for _, c := range clauses {
			all, longest = all+s.code(c), max(longest, s.code(c))
		}

		count += all - longest
		return false
	})

	return count
}

// repeated reports whether fn's body repeats one in seen, and names that one. It records a new body in seen. Only a
// body of 10 lines or more, in two statements or more, counts. So a func that only fills a struct repeats nothing.
func (s source) repeated(fn *ast.FuncDecl, seen map[uint64]string) (first string, ok bool) {
	if s.code(fn.Body) < minDuplicate || len(fn.Body.List) < 2 {
		return "", false
	}

	shape := s.shape(fn.Body)
	if first, ok = seen[shape]; !ok {
		seen[shape] = fn.Name.Name + " in " + s.path
	}

	return first, ok
}

// shape hashes body's tokens, with each name and each literal replaced by its kind. So two bodies that differ only
// in names and literals have one shape.
func (s source) shape(body *ast.BlockStmt) uint64 {
	text := s.src[s.fset.Position(body.Pos()).Offset:s.fset.Position(body.End()).Offset]
	var sc scanner.Scanner
	sc.Init(token.NewFileSet().AddFile("", -1, len(text)), text, nil, 0)
	h := fnv.New64a()
	for {
		_, tok, _ := sc.Scan()
		if tok == token.EOF {
			return h.Sum64()
		}

		_, _ = h.Write([]byte(tok.String() + " ")) // A hash.Hash never returns an error from Write.
	}
}

// cases returns the clauses of n when n is a switch, a type switch or a select.
func cases(n ast.Node) ([]ast.Stmt, bool) {
	switch s := n.(type) {
	case *ast.SwitchStmt:
		return s.Body.List, true
	case *ast.TypeSwitchStmt:
		return s.Body.List, true
	case *ast.SelectStmt:
		return s.Body.List, true
	}

	return nil, false
}

// stmts sums the cognitive complexity of list, which is nested depth levels deep.
func stmts(list []ast.Stmt, depth int) (cost int) {
	for _, s := range list {
		cost += stmt(s, depth)
	}

	return cost
}

func stmt(s ast.Stmt, depth int) int {
	if clauses, ok := cases(s); ok {
		costliest := 0
		for _, c := range clauses {
			costliest = max(costliest, clause(c, depth+1))
		}

		return 1 + depth + costliest
	}

	switch s := s.(type) {
	case *ast.IfStmt:
		return 1 + depth + exprs(depth, s.Init, s.Cond) + stmts(s.Body.List, depth+1) + otherwise(s.Else, depth)
	case *ast.ForStmt:
		return 1 + depth + exprs(depth, s.Init, s.Cond, s.Post) + stmts(s.Body.List, depth+1)
	case *ast.RangeStmt:
		return 1 + depth + exprs(depth, s.X) + stmts(s.Body.List, depth+1)
	case *ast.BlockStmt:
		return stmts(s.List, depth)
	case *ast.LabeledStmt:
		return stmt(s.Stmt, depth)
	case *ast.BranchStmt:
		if s.Label != nil {
			return 1
		}
	}

	return exprs(depth, s)
}

// otherwise is the cost of an if's else. An else costs 1, and so does an else if, whatever its depth.
func otherwise(els ast.Stmt, depth int) int {
	switch e := els.(type) {
	case *ast.IfStmt:
		return stmt(e, depth) - depth
	case *ast.BlockStmt:
		return 1 + stmts(e.List, depth+1)
	}

	return 0
}

// clause is the cost of one case of a switch or a select: its expressions and its body.
func clause(c ast.Stmt, depth int) int {
	switch c := c.(type) {
	case *ast.CaseClause:
		cost := stmts(c.Body, depth)
		for _, e := range c.List {
			cost += exprs(depth, e)
		}

		return cost
	case *ast.CommClause:
		return exprs(depth, c.Comm) + stmts(c.Body, depth)
	}

	return 0
}

// exprs is the cost of the expressions under each of nodes. A chain of && costs 1, and so does a chain of ||. So
// a && b && c costs 1, and a && b || c costs 2. A function literal costs what its body does, one level deeper.
func exprs(depth int, nodes ...ast.Node) (cost int) {
	counted := map[*ast.BinaryExpr]bool{}
	for _, node := range nodes {
		if node == nil {
			continue
		}

		ast.Inspect(node, func(n ast.Node) bool {
			switch e := n.(type) {
			case *ast.FuncLit:
				cost += stmts(e.Body.List, depth+1)
				return false
			case *ast.BinaryExpr:
				if logical(e) && !counted[e] {
					cost += runs(e, token.ILLEGAL, counted)
				}
			}

			return true
		})
	}

	return cost
}

func logical(e *ast.BinaryExpr) bool { return e.Op == token.LAND || e.Op == token.LOR }

// runs counts the chains of one operator in the tree of && and || under e. above is the operator that joins e to the
// rest of its tree. It marks each && and || of the tree in counted.
func runs(e ast.Expr, above token.Token, counted map[*ast.BinaryExpr]bool) int {
	b, ok := ast.Unparen(e).(*ast.BinaryExpr)
	if !ok || !logical(b) {
		return 0
	}

	counted[b] = true
	n := runs(b.X, b.Op, counted) + runs(b.Y, b.Op, counted)
	if b.Op != above {
		n++
	}

	return n
}

// operators counts each && and || in the tree of them under e.
func operators(e ast.Expr) int {
	b, ok := ast.Unparen(e).(*ast.BinaryExpr)
	if !ok || !logical(b) {
		return 0
	}

	return 1 + operators(b.X) + operators(b.Y)
}

// conditions yields each condition of an if, a for or a case under root.
func conditions(root ast.Node) iter.Seq[ast.Expr] {
	return func(yield func(ast.Expr) bool) {
		ast.Inspect(root, func(n ast.Node) bool {
			var conds []ast.Expr
			switch s := n.(type) {
			case *ast.IfStmt:
				conds = []ast.Expr{s.Cond}
			case *ast.ForStmt:
				conds = []ast.Expr{s.Cond}
			case *ast.CaseClause:
				conds = s.List
			}

			for _, c := range conds {
				if c != nil && !yield(c) {
					return false
				}
			}

			return true
		})
	}
}

// passesThrough reports whether fn's body is one call that hands on each of fn's parameters as it came. The call may
// add literals and other bare names. It names the callee, which is a bare function name or a method of fn's receiver.
func passesThrough(fn *ast.FuncDecl) (callee string, ok bool) {
	call := onlyCall(fn.Body)
	if call == nil {
		return "", false
	}

	own := map[string]bool{}
	for _, p := range fn.Type.Params.List {
		for _, name := range p.Names {
			own[name.Name] = true
		}
	}

	passed := 0
	for _, a := range call.Args {
		id, named := a.(*ast.Ident)
		_, literal := a.(*ast.BasicLit)
		if named && own[id.Name] {
			passed++
		} else if !named && !literal {
			return "", false
		}
	}

	callee, ok = calleeOf(fn, call)

	return callee, ok && passed > 0 && passed == len(own)
}

// onlyCall returns the call that is the whole body, or nil when body does anything else.
func onlyCall(body *ast.BlockStmt) *ast.CallExpr {
	if len(body.List) != 1 {
		return nil
	}

	var e ast.Expr
	switch s := body.List[0].(type) {
	case *ast.ReturnStmt:
		if len(s.Results) == 1 {
			e = s.Results[0]
		}
	case *ast.ExprStmt:
		e = s.X
	}

	call, _ := e.(*ast.CallExpr)
	return call
}

// calleeOf names what call calls. It returns false unless that is a bare function name, or a method of fn's
// receiver.
func calleeOf(fn *ast.FuncDecl, call *ast.CallExpr) (string, bool) {
	switch fun := call.Fun.(type) {
	case *ast.Ident:
		return fun.Name, true
	case *ast.SelectorExpr:
		if x, named := fun.X.(*ast.Ident); named {
			return x.Name + "." + fun.Sel.Name, x.Name == receiver(fn)
		}
	}

	return "", false
}

// receiver returns the name of fn's receiver, or "" when fn has none.
func receiver(fn *ast.FuncDecl) string {
	if fn.Recv == nil || len(fn.Recv.List[0].Names) == 0 {
		return ""
	}

	return fn.Recv.List[0].Names[0].Name
}
