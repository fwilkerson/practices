package main

import (
	"cmp"
	"fmt"
	"os"
	"path/filepath"
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

func TestLayout(t *testing.T) {
	for rule, tc := range map[string]layoutCase{
		"a port opens with Port": {
			path: "x.go",
			src:  "package x\n\ntype ACmd struct{}\n\ntype Port struct{}\n",
			want: []string{"5: out of order: Port. Want: Port, ACmd"},
		},
		"a port's commands follow Port's order": {
			path: "x.go",
			src:  "package x\n\ntype Port struct{}\n\ntype BCmd struct{}\n\ntype ACmd struct{}\n",
			want: []string{"7: out of order: ACmd. Want: Port, ACmd, BCmd"},
		},
		"a port's Key methods follow its requests": {
			path: "x.go",
			src:  "package x\n\ntype Port struct{}\n\nfunc (c ACmd) Key() uuid.UUID { return c.ID }\n\ntype ACmd struct{}\n",
			want: []string{"7: out of order: ACmd. Want: Port, ACmd, ACmd.Key"},
		},
		"a port's key type sits with its Key methods": {
			path: "x.go",
			src:  "package x\n\ntype Port struct{}\n\ntype R struct{}\n\ntype Ref struct{}\n\nfunc (Ref) Key() {}\n",
			want: []string{"5: out of order: R. Want: Port, Ref, Ref.Key, R"},
		},
		"every declaration out of order is reported at once": {
			path: "x.go",
			src:  "package x\n\ntype BCmd struct{}\n\ntype ACmd struct{}\n\ntype Port struct{}\n",
			want: []string{"5: out of order: ACmd, Port. Want: Port, ACmd, BCmd"},
		},
		"a declaration moves with its comment and the blank lines above it": {
			path:  "x.go",
			src:   "package x\n\n// ACmd adds.\ntype ACmd struct{}\n\ntype Port struct{}\n",
			fixed: "package x\n\ntype Port struct{}\n\n// ACmd adds.\ntype ACmd struct{}\n",
			want:  []string{"6: out of order: Port. Want: Port, ACmd"},
		},
		"a service opens with New": {
			path: "service/service.go",
			src:  "package service\n\ntype service struct{}\n\nfunc New() {}\n",
			want: []string{"5: out of order: New. Want: New, service"},
		},
		"a service's handlers follow Port's order": {
			path: "service/service.go",
			src:  "package service\n\nfunc New() {}\n\nfunc (s *service) b() {}\n\nfunc (s *service) a() {}\n",
			want: []string{"7: out of order: service.a. Want: New, service.a, service.b"},
		},
		"a declaration named after no endpoint may sit anywhere in its section": {
			path: "service/service.go",
			src:  "package service\n\nfunc New() {}\n\nfunc (s *service) a() {}\n\nfunc (s *service) b() {}\n\nfunc (s *service) c() {}\n",
		},
		"a model's events sit after its queries and before Apply": {
			path: "internal/model/model.go",
			src:  "package model\n\ntype Event = any\n\nfunc (r *R) Apply(Event) {}\n\nfunc (r *R) Total() int { return 0 }\n",
			want: []string{"7: out of order: R.Total. Want: R.Total, Event, R.Apply"},
		},
		"a declaration that belongs last says so": {
			path: "internal/model/model.go",
			src:  "package model\n\nfunc (r *R) Apply(Event) {}\n\ntype Added struct{}\n\nvar codec = NewCodec(Added{})\n",
			want: []string{"3: out of order: R.Apply. Want: Added, codec, R.Apply"},
		},
	} {
		t.Run(rule, func(t *testing.T) { checkLayout(t, tc) })
	}
}

// layoutCase is a file in module x and the problems that layout reports in it. checkLayout writes x's port beside
// it, with endpoints A and B. When fixed is not empty, it is the file that layout must return.
type layoutCase struct {
	path, src, fixed string
	want             []string
}

func checkLayout(t *testing.T, tc layoutCase) {
	const port = "package x\n\ntype Port struct {\n\tA kit.Endpoint[ACmd, R]\n\tB kit.Endpoint[BCmd, R]\n}\n"
	dir := filepath.Join(t.TempDir(), "modules", "x")
	claim.Must(t, os.MkdirAll(dir, 0o755))
	claim.Must(t, os.WriteFile(filepath.Join(dir, "x.go"), []byte(port), 0o644))
	out, problems, err := layout(filepath.Join(dir, tc.path), []byte(tc.src))
	claim.Must(t, err)

	if tc.fixed != "" && string(out) != tc.fixed {
		t.Errorf("fixed to\n%s\nwant\n%s", out, tc.fixed)
	}

	if _, again, err := layout(filepath.Join(dir, tc.path), out); err != nil || again != nil {
		t.Errorf("after the fix, got %v and %v\n%s", again, err, out)
	}

	var found []string
	for _, p := range problems {
		found = append(found, fmt.Sprintf("%d: %s", p.line, p.msg))
	}

	if !slices.Equal(found, tc.want) {
		t.Errorf("got %q, want %q", found, tc.want)
	}
}
