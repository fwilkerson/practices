package main

import (
	"strconv"
	"strings"
	"testing"

	"github.com/fwilkerson/practices/claim"
)

func TestHealth(t *testing.T) {
	// nested is six levels of for and if, which cost 1+2+3+4+5+6.
	nested := "func f(a, b int) {\n" + strings.Repeat("for range a {\nif a > b {\n", 3) + strings.Repeat("}\n", 7)
	for rule, tc := range map[string]struct {
		src  string
		want []string
	}{
		"each level of nesting costs one more": {
			src:  nested,
			want: []string{"f has a cognitive complexity of 21, over 15"},
		},
		"a switch counts as its costliest case": {
			src: "func f(v any, a int) {\nswitch v.(type) {\n" + typeCases(20, "if a > %d {\na++\n}\n") + "}\n}\n",
		},
		"an else costs one, whatever its depth": {
			src: "func f(a int) {\nif a > 0 {\na++\n}" + strings.Repeat(" else if a > 1 {\na++\n}", 14) + "\n}\n",
		},
		"a function holds at most 80 lines of code": {
			src:  "func f(a int) {\n" + strings.Repeat("a++\n", 79) + "}\n",
			want: []string{"f holds 81 lines of code, over 80"},
		},
		"a blank line and a comment are not code": {
			src: "func f(a int) {\n" + strings.Repeat("a++\n\n// A comment.\n", 78) + "}\n",
		},
		"a switch counts as its longest case": {
			src: "func f(v any, a int) {\nswitch v.(type) {\n" + typeCases(40, "a++\na--\n") + "}\n}\n",
		},
		"a condition holds at most one && or ||": {
			src:  "func f(a, b, c bool) {\nif a && b || c {\nreturn\n}\n}\n",
			want: []string{"f has a condition with 2 of && and ||"},
		},
		"each expression of a case is a condition": {
			src:  "func f(a, b, c bool) {\nswitch {\ncase a && b, c:\ncase a || b || c:\n}\n}\n",
			want: []string{"f has a condition with 2 of && and ||"},
		},
		"a chain inside a call's argument costs too": {
			src:  "func f(a, b bool) {\n" + strings.Repeat("if a && g(a || b) {\nreturn\n}\n", 6) + "}\n",
			want: []string{"f has a cognitive complexity of 18, over 15"},
		},
		"a returned expression is no condition": {
			src: "func f(a, b, c bool) bool {\nreturn a && b && c\n}\n",
		},
		"a function takes at most 4 parameters": {
			src:  "func f(a, b int, c, d string, e bool) {}\n",
			want: []string{"f takes 5 parameters, over 4"},
		},
		"ten parameters cost two points": {
			src:  "func f(a, b, c, d, e, f, g, h, i, j int) {}\n",
			want: []string{"f takes 10 parameters, over 4 (2 points)"},
		},
		"a body repeats another but for its names and literals": {
			src: "func f(a int) int {\n" + strings.Repeat("a += 1\n", 9) + "return a\n}\n\n" +
				"func g(b int) int {\n" + strings.Repeat("b += 2\n", 9) + "return b\n}\n",
			want: []string{"g repeats the body of f in p.go"},
		},
		"a body of one statement repeats nothing": {
			src: "func f() T {\nreturn T{\n" + strings.Repeat("A: 1,\n", 10) + "}\n}\n\n" +
				"func g() T {\nreturn T{\n" + strings.Repeat("A: 2,\n", 10) + "}\n}\n",
		},
		"a function only hands its parameters on": {
			src:  "func f(a int) int {\nreturn g(a, 0)\n}\n\nfunc (t T) m(a int) {\nt.n(a)\n}\n",
			want: []string{"f only hands its parameters on to g", "m only hands its parameters on to t.n"},
		},
		"a call that adds a value is no pass-through": {
			src: "func f(a int) int {\nreturn g(a, h())\n}\n\nfunc (t T) m(a int) {\nt.store.n(a)\n}\n",
		},
	} {
		t.Run(rule, func(t *testing.T) {
			failures, err := checkFile("p.go", []byte("package p\n\n"+tc.src), map[uint64]string{})
			claim.Must(t, err)

			var got []string
			for _, f := range failures {
				got = append(got, f.msg+strings.Repeat(" (2 points)", f.cost-1))
			}

			if diff := claim.Diff(tc.want, got); diff != "" {
				t.Errorf("checkFile's failures differ (-want +got):\n%s", diff)
			}
		})
	}
}

// typeCases returns n cases of a type switch, each on its own type, with body as its statements. A %d in body gets the
// case's number.
func typeCases(n int, body string) string {
	var b strings.Builder
	for i := range n {
		b.WriteString("case T" + strconv.Itoa(i) + ":\n" + strings.ReplaceAll(body, "%d", strconv.Itoa(i)))
	}

	return b.String()
}
