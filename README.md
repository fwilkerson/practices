# practices

A guide to writing and judging Go code, [guide.md](guide.md), with the commands and test helpers that hold a project to
the rules a tool can count. A project adds this module to its `go.mod`, and `go test` fails where the code breaks one.

```text
go.mod tool lines     go tool guide     prints guide.md
                      go tool health    scores each file from 10 to 1, by six counted rules
                      go tool style     the rules gofmt leaves out, fixed where it can
                      go tool size      lines of code, test and comment for each directory
                      go tool inspect   GoLand's inspections, one line for each finding

root test             repotest          one line for each check over the whole repository
package tests         claim             Holds, Must and Diff, for got/want failures
```

It needs Go 1.27.2 or later. `go doc -all` on each package is its contract.

## Add the commands

```text
$ go get -tool github.com/fwilkerson/practices/cmd/...@v0.1.0
go: downloading github.com/fwilkerson/practices v0.1.0
go: added github.com/fwilkerson/practices v0.1.0
```

That adds the module, and one `tool` line for each command:

```text
tool (
	github.com/fwilkerson/practices/cmd/guide
	github.com/fwilkerson/practices/cmd/health
	github.com/fwilkerson/practices/cmd/inspect
	github.com/fwilkerson/practices/cmd/size
	github.com/fwilkerson/practices/cmd/style
)
```

## Run them

Each sample below ran on lines 10 to 21 of this file, `order/order.go`:

```go
// Total returns the sum of o's items, less the discount when the order is large and paid, or the customer is new.
func Total(o Order, discount, minItems int, isNew, staff bool) int {
	sum := 0
	for _, item := range o.Items {
		sum += item
	}
	if len(o.Items) > minItems && o.Paid || isNew {
		return sum - discount
	}

	return sum
}
```

```text
$ go tool guide | head -1
# Writing and Judging a Go API

$ go tool health .
order/order.go:11: Total takes 5 parameters, over 4
order/order.go:16: Total has a condition with 2 of && and ||
 8  order/order.go

$ go tool style -l .
order/order.go:10: a sentence in this comment has more than 20 words
order/order.go:16: a blank line follows a closing brace

$ go tool style .
order/order.go:10: a sentence in this comment has more than 20 words

$ go tool size .
        code   fake   test  bench  comment  test%  comment%
docs       0      0      0      0        0      -         -
order     15      0     13      0        3    86%       10%
*.go       0      0     12      0        1      -        8%
.         15      0     25      0        4   166%       10%

$ go tool inspect
.../shop/order/order.go:11: Unused function <code>Total</code> (GoUnusedExportedFunction)
.../shop/order/order.go:11: Unused parameter <code>staff</code> (GoUnusedParameter)
```

Health, style -l and inspect exit 1 when they find something. Style without -l fixes the formatting and the blank
lines in place, and lists what it left. Inspect runs on macOS alone, and needs a licensed GoLand in
`/Applications/GoLand.app`, or the install that its -app flag names. The other commands need only Go.

## Hold the repository to them

A test file at the repository's root calls each check of [`repotest`](repotest) in a line:

```go
package shop_test

import (
	"testing"

	"github.com/fwilkerson/practices/repotest"
)

func TestStyle(t *testing.T)  { repotest.Style(t) }
func TestModern(t *testing.T) { repotest.Modern(t) }
func TestGuide(t *testing.T)  { repotest.Guide(t, "docs/guide.md") }
func TestNames(t *testing.T)  { repotest.Names(t, "README.md") }
func TestLinks(t *testing.T)  { repotest.Links(t, "README.md") }
func TestHealth(t *testing.T) { repotest.Health(t, held) }

// held is the score of each file under 10. Each stays until its failures are fixed.
var held = map[string]string{}
```

Each failure names its fix:

```text
$ go test .
--- FAIL: TestGuide (0.00s)
    shop_test.go:11: docs/guide.md differs from the guide, open docs/guide.md: no such file or directory: run go tool guide > docs/guide.md
--- FAIL: TestLinks (0.00s)
    shop_test.go:13: README.md:3: the link to docs/guide.md names no file. Fix the path, or drop the link.
--- FAIL: TestHealth (0.10s)
    shop_test.go:14: go tool health's scores differ (-held +got):
          map[string]string{
        + 	"order/order.go": "8",
          }
...
```

`repotest.Style` and `repotest.Health` run the commands, so they need the `tool` lines above. `repotest.Context` caps
the words of a file that every Claude Code session loads, such as `CLAUDE.md`.

The project keeps a copy of the guide, so its reviewers and agents read the rules by a path. Write it with
`go tool guide > docs/guide.md`, and run that again after each upgrade of this module. `repotest.Guide` fails until
you do.

## Check a result in a test

Package [`claim`](claim) writes a check in Go's got/want form:

```go
func TestTotal(t *testing.T) {
	o := order.Order{Items: []int{3, 4}, Paid: true}
	got := order.Total(o, 2, 1, false, false)
	if !claim.Holds(got == 5, len(o.Items) == 2) {
		t.Errorf("Total = %d; want 5", got)
	}
}
```

`claim.Must` stops a test on a setup error, and `claim.Diff` returns how two values differ.

## License

Copyright 2026 Frank Wilkerson. Licensed under the Apache License, Version 2.0, in [LICENSE](LICENSE).
