// Command style enforces the rules that gofmt leaves out, so no reviewer has to judge them. With -l it changes
// nothing, lists what is wrong, and exits 1 if anything is:
//
//	$ go tool style -l .
//	order/order.go:10: a sentence in this comment has more than 20 words
//	order/order.go:16: a blank line follows a closing brace
//
// Without -l it fixes formatting and blank lines in place. It lists only the wide lines and long sentences left,
// and exits 1 if it lists one:
//
//	$ go tool style .
//	order/order.go:10: a sentence in this comment has more than 20 words
//
// The rules:
//
//   - A file is formatted as gofmt formats it.
//   - An if that checks what the statement above it assigned sits directly under it.
//   - Otherwise a blank line follows a block's closing brace.
//   - A line is at most 120 columns wide, a tab counting as 4. A row of a table may be wider.
//   - A sentence in a comment has at most 20 words. A list item, an indented line or an example's output may be
//     longer.
//
// An if with no init checks the statement above it when its condition uses a name that statement assigns.
// A table is a composite literal over several lines, and each element starts a row. A sentence ends at a word that
// ends in a period, a question mark or an exclamation mark. A block comment and a directive such as //go:embed are not
// read.
//
// It reads each directory it is given, or the working directory. It skips the directories that ./... skips:
// testdata, vendor, and each name that starts with . or _.
//
// Where to break a long run of statements into steps is still the writer's call.
package main

import (
	"flag"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

const (
	maxWidth = 120
	maxWords = 20
)

type problem struct {
	line  int
	msg   string
	fixed bool
}

func main() {
	list := flag.Bool("l", false, "list what is wrong and change nothing")
	flag.Parse()
	roots := flag.Args()
	if len(roots) == 0 {
		roots = []string{"."}
	}

	wrong := false
	for _, root := range roots {
		err := walk(root, func(path string) error {
			printed, err := fix(path, *list)
			wrong = wrong || printed

			return err
		})
		if err != nil {
			log.Fatal(err)
		}
	}

	if wrong {
		os.Exit(1)
	}
}

// walk calls visit on each Go file under root, leaving out the directories that ./... leaves out.
func walk(root string, visit func(path string) error) error {
	return filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		switch {
		case err != nil:
			return err
		case d.IsDir() && skip(root, path):
			return filepath.SkipDir
		case d.IsDir() || !strings.HasSuffix(path, ".go"):
			return nil
		}

		return visit(path)
	})
}

// skip reports whether ./... leaves out the directory at path, under root. It leaves out testdata, vendor, and names
// that start with . or _, but never root itself.
func skip(root, path string) bool {
	dir := filepath.Base(path)
	return path != root &&
		(strings.HasPrefix(dir, ".") || strings.HasPrefix(dir, "_") || dir == "testdata" || dir == "vendor")
}

// fix writes the file at path styled, and prints each problem it cannot fix. With list, it prints every problem and
// writes nothing. It reports whether it printed any.
func fix(path string, list bool) (bool, error) {
	src, err := os.ReadFile(path)
	if err != nil {
		return false, err
	}

	out, problems, err := style(path, src)
	if err != nil {
		return false, err
	}

	printed := false
	for _, p := range problems {
		if list || !p.fixed {
			fmt.Printf("%s:%d: %s\n", path, p.line, p.msg)
			printed = true
		}
	}

	if list || string(out) == string(src) {
		return printed, nil
	}

	return printed, os.WriteFile(path, out, 0o644)
}

const (
	needsBlank = "a blank line follows a closing brace"
	extraBlank = "an if sits directly under the statement it checks"
	tooWide    = "the line is wider than 120 columns"
	tooLong    = "a sentence in this comment has more than 20 words"
	notGofmt   = "the file is not formatted as gofmt formats it"
)

// style returns src with its blank lines and formatting fixed, and the problems it found. Each problem's line is in
// src.
func style(path string, src []byte) ([]byte, []problem, error) {
	out, problems, err := spacing(path, src)
	if err != nil {
		return nil, nil, err
	}

	formatted, err := format.Source(out)
	if err != nil {
		return nil, nil, err
	}

	if string(formatted) != string(out) {
		problems = append(problems, problem{1, notGofmt, true})
	}

	slices.SortFunc(problems, func(p, q problem) int { return p.line - q.line })
	return formatted, problems, nil
}

// spacing returns src with its blank lines fixed, and the problems it found.
func spacing(path string, src []byte) ([]byte, []problem, error) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, path, src, parser.ParseComments)
	if err != nil {
		return nil, nil, err
	}

	lines := strings.Split(string(src), "\n")
	line := func(p token.Pos) int { return fset.Position(p).Line }
	out, problems := blankLines(f, line, lines)
	rows := tableRows(f, line)
	for i, l := range lines {
		if width(l) > maxWidth && !rows[i+1] {
			problems = append(problems, problem{i + 1, tooWide, false})
		}
	}

	for _, n := range longSentences(f, line) {
		problems = append(problems, problem{n, tooLong, false})
	}

	return []byte(strings.Join(out, "\n")), problems, nil
}

// blankLines returns lines with a blank line added after each closing brace that needs one. It drops each blank line
// above an if that checks the statement before it. It also returns a problem for each fix.
func blankLines(f *ast.File, line func(token.Pos) int, lines []string) ([]string, []problem) {
	var problems []problem
	var drop []int
	filled := func(n int) bool { return strings.TrimSpace(lines[n-1]) != "" }
	addAfter := map[int]bool{}
	for a, b := range pairs(f) {
		end, next := line(a.End()), line(b.Pos())
		blanks := slices.DeleteFunc(lineRange(end+1, next), filled)
		checked := checks(a, b)
		switch {
		case checked && len(blanks) > 0:
			problems = append(problems, problem{next, extraBlank, true})
			drop = append(drop, blanks...)
		case checked:
			// An if stays against the statement it checks, even where that statement ends in a closing brace.
		case closesBlock(a, line, lines) && len(blanks) == 0:
			problems = append(problems, problem{next, needsBlank, true})
			addAfter[end] = true
		}
	}

	return edit(lines, drop, addAfter), problems
}

// edit returns lines without those in drop, and with a blank line after each that addAfter marks. Both count lines
// from 1.
func edit(lines []string, drop []int, addAfter map[int]bool) []string {
	var out []string
	for i, l := range lines {
		if !slices.Contains(drop, i+1) {
			out = append(out, l)
		}

		if addAfter[i+1] {
			out = append(out, "")
		}
	}

	return out
}

// pairs yields each two adjacent statements, a then b, in every block and in every case's body. It skips a pair of
// case clauses, which are the statements in the block of a switch or a select.
func pairs(f *ast.File) func(yield func(a, b ast.Stmt) bool) {
	return func(yield func(a, b ast.Stmt) bool) {
		ast.Inspect(f, func(n ast.Node) bool {
			var list []ast.Stmt
			switch n := n.(type) {
			case *ast.BlockStmt:
				list = n.List
			case *ast.CaseClause:
				list = n.Body
			case *ast.CommClause:
				list = n.Body
			}

			for i := 1; i < len(list); i++ {
				if clause(list[i]) {
					continue
				}

				if !yield(list[i-1], list[i]) {
					return false
				}
			}

			return true
		})
	}
}

func clause(s ast.Stmt) bool {
	switch s.(type) {
	case *ast.CaseClause, *ast.CommClause:
		return true
	}

	return false
}

func lineRange(from, to int) []int {
	var r []int
	for n := from; n < to; n++ {
		r = append(r, n)
	}

	return r
}

// closesBlock reports whether s ends with a block's closing brace, as an if, a for or a func literal does. The brace
// must start its line. A composite literal's closing brace does not count.
func closesBlock(s ast.Stmt, line func(token.Pos) int, lines []string) bool {
	end := line(s.End())
	if !strings.HasPrefix(strings.TrimSpace(lines[end-1]), "}") {
		return false
	}

	found := false
	ast.Inspect(s, func(n ast.Node) bool {
		if b, ok := n.(*ast.BlockStmt); ok && line(b.Rbrace) == end {
			found = true
		}

		return !found
	})

	return found
}

// checks reports whether next is an if statement, with no init, whose condition uses a name that prev assigns.
func checks(prev, next ast.Stmt) bool {
	as, ok := prev.(*ast.AssignStmt)
	is, isIf := next.(*ast.IfStmt)

	return ok && isIf && is.Init == nil && uses(is.Cond, assigned(as))
}

// assigned returns the names on the left of as, except the blank identifier.
func assigned(as *ast.AssignStmt) map[string]bool {
	names := map[string]bool{}
	for _, e := range as.Lhs {
		if id, ok := e.(*ast.Ident); ok && id.Name != "_" {
			names[id.Name] = true
		}
	}

	return names
}

// uses reports whether e names any of names.
func uses(e ast.Expr, names map[string]bool) bool {
	used := false
	ast.Inspect(e, func(n ast.Node) bool {
		if id, ok := n.(*ast.Ident); ok && names[id.Name] {
			used = true
		}

		return !used
	})

	return used
}

// tableRows returns the line each element starts on, in every composite literal that spans more than one line.
func tableRows(f *ast.File, line func(token.Pos) int) map[int]bool {
	rows := map[int]bool{}
	ast.Inspect(f, func(n ast.Node) bool {
		if c, ok := n.(*ast.CompositeLit); ok && line(c.Lbrace) != line(c.Rbrace) {
			for _, e := range c.Elts {
				rows[line(e.Pos())] = true
			}
		}

		return true
	})

	return rows
}

// longSentences returns the line each comment sentence of more than maxWords words starts on. It skips a directive,
// such as //go:embed, and a line indented past one space, as a list item is. It also skips an example's output.
func longSentences(f *ast.File, line func(token.Pos) int) []int {
	s := sentences{line: line}
	for _, g := range f.Comments {
		if output(g) {
			continue
		}

		for _, c := range g.List {
			s.read(c)
		}

		s.end()
	}

	return s.long
}

// prose returns c's text, and reports whether the limit of maxWords applies to it. The limit skips a directive, such
// as //go:embed, and a line indented past one space.
func prose(c *ast.Comment) (string, bool) {
	text, ok := strings.CutPrefix(c.Text, "// ")
	return text, ok && !strings.HasPrefix(text, " ") && !strings.HasPrefix(text, "\t")
}

// sentences counts the words of each sentence in a comment. long lists the first line of each sentence over maxWords.
// start is the first line of the current sentence, and words is its count so far.
type sentences struct {
	line         func(token.Pos) int
	long         []int
	start, words int
}

// read counts the words of c, one line of a comment. A line that is not prose ends the sentence.
func (s *sentences) read(c *ast.Comment) {
	text, ok := prose(c)
	if !ok {
		s.end()
		return
	}

	for w := range strings.FieldsSeq(text) {
		if s.words == 0 {
			s.start = s.line(c.Pos())
		}

		s.words++
		if t := strings.TrimRight(w, `"')`); t != "" && strings.ContainsRune(".?!", rune(t[len(t)-1])) {
			s.end()
		}
	}
}

// end ends the current sentence, and appends its first line to long if it has more than maxWords words.
func (s *sentences) end() {
	if s.words > maxWords {
		s.long = append(s.long, s.start)
	}

	s.words = 0
}

// output reports whether g is the output an example must print. go test compares it, and go doc does not show it.
func output(g *ast.CommentGroup) bool {
	first := strings.ToLower(g.List[0].Text)
	return strings.HasPrefix(first, "// output:") || strings.HasPrefix(first, "// unordered output:")
}

func width(l string) int {
	return len([]rune(l)) + 3*strings.Count(l, "\t")
}
