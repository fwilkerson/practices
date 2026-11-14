// Command size counts the lines of Go under each directory it is given. It prints a row for each subdirectory, a row
// for the directory's own files, and a total. Run it again on a large row to see inside it:
//
//	go tool size modules internal cmd
//	go tool size modules/order
//
// It skips the directories that ./... skips, such as testdata. It classifies each line as cloc does. A line with any
// code is code, and a line with only a comment is comment. Each line of code goes in one of four columns:
//
//   - bench counts the files whose name ends in _bench_test.go, which hold the benchmarks.
//   - test counts the other _test.go files, and the files of a directory whose name ends in test, such as kittest.
//   - fake counts each fake.go, which stands in for an outside system when a program runs locally.
//   - code counts the rest.
//
// The comment column counts the comment lines of every file. An example_test.go file is documentation, checked when
// the tests run, so its lines of code count as comment too.
//
// test% is test lines per 100 code lines. comment% is comment lines per 100 lines of code, fake, test and bench.
package main

import (
	"bytes"
	"flag"
	"fmt"
	"go/scanner"
	"go/token"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"
)

type count struct {
	code, fake, test, bench, comment int
}

func (c *count) add(o count) {
	c.code += o.code
	c.fake += o.fake
	c.test += o.test
	c.bench += o.bench
	c.comment += o.comment
}

type row struct {
	name string
	count
}

func main() {
	flag.Parse()
	roots := flag.Args()
	if len(roots) == 0 {
		roots = []string{"."}
	}

	var rows []row
	var all count
	for _, root := range roots {
		children, total, err := tally(root)
		if err != nil {
			log.Fatal(err)
		}

		rows = append(rows, children...)
		rows = append(rows, row{root, total}, row{})
		all.add(total)
	}

	if len(roots) > 1 {
		rows = append(rows, row{"total", all})
	}

	report(rows)
}

// tally returns a row for each subdirectory of root. A last row holds root's own Go files, if it has any. It also
// returns root's total.
func tally(root string) ([]row, count, error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, count{}, err
	}

	own := row{name: filepath.Join(root, "*.go")}
	var rows []row
	var total count
	for _, e := range entries {
		if skip(e) {
			continue
		}

		path := filepath.Join(root, e.Name())
		c, err := tree(path)
		if err != nil {
			return nil, count{}, err
		}

		if e.IsDir() {
			rows = append(rows, row{path, c})
		} else {
			own.add(c)
		}

		total.add(c)
	}

	if own.count != (count{}) {
		rows = append(rows, own)
	}

	return rows, total, nil
}

// tree counts every Go file under root, which may itself be a file.
func tree(root string) (total count, err error) {
	err = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		switch {
		case err != nil:
			return err
		case path != root && skip(d):
			return filepath.SkipDir
		case d.IsDir() || !strings.HasSuffix(path, ".go"):
			return nil
		}

		c, err := file(path)
		total.add(c)

		return err
	})

	return total, err
}

// skip reports whether e is a directory that ./... skips: testdata, vendor, or a name that starts with . or _.
func skip(e fs.DirEntry) bool {
	dir := e.Name()
	return e.IsDir() &&
		(strings.HasPrefix(dir, ".") || strings.HasPrefix(dir, "_") || dir == "testdata" || dir == "vendor")
}

func file(path string) (count, error) {
	src, err := os.ReadFile(path)
	if err != nil {
		return count{}, err
	}

	c, err := lines(src)
	if err != nil {
		return count{}, fmt.Errorf("%s: %w", path, err)
	}

	return codeToColumn(c, path), nil
}

// codeToColumn moves c's code lines to the column for path's kind of file: comment, bench, test or fake. Any other
// file keeps them as code.
func codeToColumn(c count, path string) count {
	switch {
	case filepath.Base(path) == "example_test.go":
		c.comment, c.code = c.comment+c.code, 0
	case strings.HasSuffix(path, "_bench_test.go"):
		c.bench, c.code = c.code, 0
	case strings.HasSuffix(path, "_test.go") || strings.HasSuffix(filepath.Base(filepath.Dir(path)), "test"):
		c.test, c.code = c.code, 0
	case filepath.Base(path) == "fake.go":
		c.fake, c.code = c.code, 0
	}

	return c
}

// lines counts each line of src as code, comment or blank. A line with any code is code. A line with only a comment
// is comment.
func lines(src []byte) (count, error) {
	kinds, err := lineKinds(src)
	if err != nil {
		return count{}, err
	}

	var c count
	for _, k := range kinds {
		switch k {
		case codeLine:
			c.code++
		case commentLine:
			c.comment++
		}
	}

	return c, nil
}

// A line that is not blank is one of these kinds. codeLine is the greater, so a line with both counts as code.
const (
	commentLine = iota + 1
	codeLine
)

// lineKinds returns the kind of each line of src, indexed by line number from 1. A blank line's kind is 0. The slice
// may end with a blank entry past the last line.
func lineKinds(src []byte) ([]int, error) {
	kinds := make([]int, bytes.Count(src, []byte("\n"))+2)

	var errs scanner.ErrorList
	f := token.NewFileSet().AddFile("", -1, len(src))
	var s scanner.Scanner

	s.Init(f, src, errs.Add, scanner.ScanComments)
	for {
		pos, tok, lit := s.Scan()
		if tok == token.EOF {
			break
		}

		if tok == token.SEMICOLON && lit == "\n" {
			continue // The scanner inserted this semicolon, so no one wrote it.
		}

		kind := codeLine
		if tok == token.COMMENT {
			kind = commentLine
		}

		first := f.Line(pos)
		for l := first; l <= first+strings.Count(lit, "\n"); l++ {
			kinds[l] = max(kinds[l], kind)
		}
	}

	return kinds, errs.Err()
}

func report(rows []row) {
	width := len("total")
	for _, r := range rows {
		width = max(width, len(r.name))
	}

	format := "%-*s %6v %6v %6v %6v %8v %6v %9v\n"

	fmt.Printf(format, width, "", "code", "fake", "test", "bench", "comment", "test%", "comment%")
	for _, r := range rows {
		if r.name == "" {
			fmt.Println()
			continue
		}

		fmt.Printf(format, width, r.name, r.code, r.fake, r.test, r.bench, r.comment,
			percent(r.test, r.code), percent(r.comment, r.code+r.fake+r.test+r.bench))
	}
}

func percent(n, of int) string {
	if of == 0 {
		return "-"
	}

	return fmt.Sprintf("%d%%", 100*n/of)
}
