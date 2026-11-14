# practices

What the developer's Go projects share: how the code is written and judged, the commands that hold the rules a
tool can count, and the checks their tests share. It began as odh's own, copied from odh at `8bc3289`.

- [`guide.md`](guide.md): Writing and Judging a Go API. [`cmd/guide`](cmd/guide) prints it. Each project's own
  `docs/practices.md` adds its own rules.
- [`claim`](claim): the checks a project's tests share. `go doc -all ./claim` is its contract.
- [`cmd/style`](cmd/style): the rules gofmt leaves out, such as a blank line after a block and a comment's 20 words.
- [`cmd/health`](cmd/health): a score for each Go file, from six rules a tool can count.
- [`cmd/size`](cmd/size): the lines of code, test and comment under a directory.

## Use it from a project

A project builds against this checkout as it stands, with no version. Its `go.mod` names the checkout by its full
path, since a build in a worktree elsewhere must still find it:

```text
require github.com/fwilkerson/practices v0.0.0

replace github.com/fwilkerson/practices => /Users/dev/github.com/fwilkerson/practices

tool (
	github.com/fwilkerson/practices/cmd/guide
	github.com/fwilkerson/practices/cmd/health
	github.com/fwilkerson/practices/cmd/size
	github.com/fwilkerson/practices/cmd/style
)
```

Then `go tool guide` prints the guide, `go tool style -l .`, `go tool health .` and `go tool size .` run each
command, and a test imports `github.com/fwilkerson/practices/claim`.

## Load the guide in each session

An agent follows best what is in its context before its first turn, and `CLAUDE.md` is what puts it there. So a
project keeps a copy of the guide, and a test holds the copy to the module:

1. Run `go tool guide > docs/guide.md`, and commit the file.
2. Put `@docs/guide.md` on a line of its own in the project's `CLAUDE.md`.
3. Add a test that fails while `docs/guide.md` differs from what `go tool guide` prints, and that names the command
   in step 1 as the fix.

After each upgrade of this module, run step 1 again. The test fails until you do.

Three other ways were tried on 2026-11-14, in a session with no terminal. A `CLAUDE.md` import of a path outside
the project did not load. A hook at the session's start cut the guide at about 10,000 characters. An instruction
to run `go tool guide` loads nothing until the agent chooses to run it.

A change here reaches each project at its next build, uncommitted edits included. Run `go test ./...` here before
you leave a change in place.
