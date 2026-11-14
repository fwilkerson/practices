package main

import (
	"cmp"
	"slices"
	"strings"
	"testing"

	"github.com/fwilkerson/practices/claim"
)

func TestStyle(t *testing.T) {
	for rule, tc := range map[string]styleCase{
		"a blank line follows a closing brace": {
			body:     "if ok {\n\treturn\n}\nf()",
			want:     "if ok {\n\treturn\n}\n\nf()",
			problems: []string{needsBlank},
		},
		"a func literal's brace counts": {
			body:     "defer func() {\n\tf()\n}()\nf()",
			want:     "defer func() {\n\tf()\n}()\n\nf()",
			problems: []string{needsBlank},
		},
		"cases are not statements": {
			body: "switch {\ncase ok:\n\tif ok {\n\t\treturn\n\t}\ndefault:\n}",
		},
		"a composite literal's brace does not": {
			body: "x := []int{\n\t1,\n}\nf(x)",
		},
		"an if sits under what it checks": {
			body:     "x, err := f()\n\nif err != nil {\n\treturn\n}",
			want:     "x, err := f()\nif err != nil {\n\treturn\n}",
			problems: []string{extraBlank},
		},
		"an if sits under what it checks, even after a brace": {
			body:     "err := walk(func() {\n\tf()\n})\n\nif err != nil {\n\treturn\n}",
			want:     "err := walk(func() {\n\tf()\n})\nif err != nil {\n\treturn\n}",
			problems: []string{extraBlank},
		},
		"other blank lines are the writer's call": {
			body: "x := f()\n\ng(x)\ny := f()\nif ok {\n\treturn\n}",
		},
		"a line is at most 120 columns": {
			body:     "f(\"" + strings.Repeat("x", 120) + "\")",
			problems: []string{tooWide},
		},
		"a file is formatted as gofmt formats it": {
			body:     "x :=  f()\ng(x)",
			want:     "x := f()\ng(x)",
			problems: []string{notGofmt},
		},
		"a comment sentence has at most 20 words": {
			body:     "// " + strings.Repeat("word ", 20) + "more.\nf()",
			problems: []string{tooLong},
		},
		"a sentence may end a line and start the next": {
			body: "// " + strings.Repeat("word ", 10) + "end. Then\n// " + strings.Repeat("word ", 10) + "end.\nf()",
		},
		"a list item may be longer": {
			body: "//   - " + strings.Repeat("word ", 15) + "word\n//     " + strings.Repeat("word ", 15) + "word.\nf()",
		},
		"an example's output may be longer": {
			body: "// Output:\n// " + strings.Repeat("word ", 20) + "more\nf()",
		},
		"a table row may be wider": {
			body: "_ = []string{\n\t\"" + strings.Repeat("x", 120) + "\",\n}",
		},
	} {
		t.Run(rule, func(t *testing.T) { checkStyle(t, tc) })
	}
}

// styleCase is a function body and the problems that style reports in it. want is the body that style returns. An
// empty want means style returns body unchanged.
type styleCase struct {
	body, want string
	problems   []string
}

func checkStyle(t *testing.T, tc styleCase) {
	want := cmp.Or(tc.want, tc.body)
	out, problems, err := style("p.go", []byte(wrap(tc.body)))
	claim.Must(t, err)

	var found []string
	for _, p := range problems {
		found = append(found, p.msg)
	}

	if got := string(out); got != wrap(want) || !slices.Equal(found, tc.problems) {
		t.Errorf("got\n%s%q\nwant\n%s%q", got, found, wrap(want), tc.problems)
	}
}

// wrap returns the source of a Go file whose one function has body as its body.
func wrap(body string) string {
	lines := strings.Split(body, "\n")
	for i, l := range lines {
		if l != "" {
			lines[i] = "\t" + l
		}
	}

	return "package p\n\nfunc _() {\n" + strings.Join(lines, "\n") + "\n}\n"
}
