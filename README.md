# practices

What the developer's Go projects share: how the code is written and judged, the commands that hold the rules a
tool can count, and the checks their tests share. It began as odh's own, copied from odh at `8bc3289`.

- [`guide.md`](guide.md): Writing and Judging a Go API. [`cmd/guide`](cmd/guide) prints it. Each project's own
  `docs/practices.md` adds its own rules.
- [`claim`](claim): the checks a project's tests share. `go doc -all ./claim` is its contract.
- [`repotest`](repotest): the tests that hold a whole repository to the guide's "Mechanical checks". A project's
  root test calls each in a line. `go doc -all ./repotest` is its contract.
- [`cmd/style`](cmd/style): the rules gofmt leaves out, such as a blank line after a block and a comment's 20 words.
- [`cmd/health`](cmd/health): a score for each Go file, from six rules a tool can count.
- [`cmd/size`](cmd/size): the lines of code, test and comment under a directory.
- [`cmd/inspect`](cmd/inspect): GoLand's inspections over a project, one line for each finding. It needs GoLand
  installed, and the project's `.idea`, which holds the inspection profile.

## Use it from a project

A project builds against this checkout as it stands, with no version. Its `go.mod` names the checkout by its full
path, since a build in a worktree elsewhere must still find it:

```text
require github.com/fwilkerson/practices v0.0.0

replace github.com/fwilkerson/practices => /Users/dev/github.com/fwilkerson/practices

tool (
	github.com/fwilkerson/practices/cmd/guide
	github.com/fwilkerson/practices/cmd/health
	github.com/fwilkerson/practices/cmd/inspect
	github.com/fwilkerson/practices/cmd/size
	github.com/fwilkerson/practices/cmd/style
)
```

Then `go tool guide` prints the guide, `go tool style -l .`, `go tool health .`, `go tool size .` and
`go tool inspect` run each command, and a test imports `github.com/fwilkerson/practices/claim`.

The project's root test holds it to the rules, in a file such as `style_test.go`:

```go
func TestStyle(t *testing.T)  { repotest.Style(t) }
func TestModern(t *testing.T) { repotest.Modern(t) }
func TestGuide(t *testing.T)  { repotest.Guide(t, "docs/guide.md") }
func TestNames(t *testing.T)  { repotest.Names(t, "README.md", "docs/practices.md") }
func TestLinks(t *testing.T)  { repotest.Links(t, "README.md", "docs/practices.md") }
func TestHealth(t *testing.T) { repotest.Health(t, held) }

// TestContext holds the words that every session loads.
func TestContext(t *testing.T) { repotest.Context(t, map[string]int{"CLAUDE.md": 300}) }

// held is the score of each file under 10. Each stays until its failures are fixed.
var held = map[string]string{}
```

## Keep a copy of the guide in each project

The agents that judge a project's code, such as an audit, read the guide by its path. So a project keeps a copy of
the guide, and a test holds the copy to the module:

1. Run `go tool guide > docs/guide.md`, and commit the file.
2. Name `docs/guide.md` in the prompt of each agent that judges by it.
3. Add the test `repotest.Guide(t, "docs/guide.md")`. It fails while the file differs from the guide, and names
   the command in step 1 as the fix.

After each upgrade of this module, run step 1 again. The test fails until you do.

A project may also load the copy into every session, with `@docs/guide.md` on a line of its own in its `CLAUDE.md`.
Each session and each subagent then carries the whole guide. odh stopped doing so on 2026-11-15, since its audit
holds each build to the guide.

Three other ways to load the guide into every session were tried on 2026-11-14, in a session with no terminal. A
`CLAUDE.md` import of a path outside the project did not load. A hook at the session's start cut the guide at about
10,000 characters. An instruction to run `go tool guide` loads nothing until the agent chooses to run it.

A change here reaches each project at its next build, uncommitted edits included. Run `go test ./...` here before
you leave a change in place.
