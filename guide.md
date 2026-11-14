# Writing and Judging a Go API

The rules a project's Go code is written and judged by. A project's own `docs/practices.md` adds its rules, and
decides where the two differ.

## Principles

- Keep the code small and simple. Measure small in lines. Simple is not the same as easy.
- Make an impossible state impossible to build, rather than checking for it at runtime.
- One small reusable piece beats the same glue repeated in each caller.
- Prefer a deep module: a small interface over a lot of behavior.
- Don't make the client do anything the module could do. When in doubt, leave it out.
- Minimize conceptual weight, not the count of types or methods.
- Build what the current step needs. Expect names to change and refactors to follow.
- Use modern Go where it removes code: generics, `http.ServeMux` patterns, `log/slog`, `errors.AsType`,
  `testing/synctest`, `sync.WaitGroup.Go`. A test that fails while `go fix` would change the code keeps it modern.

## Packages

A package is a contract its users read through `go doc`, or a piece two packages share.

- Default to fewer packages. Each split adds an import, a package doc, and an exported name for each shared thing.
- Split a package when a second importer needs the code, or for a subsystem whose contract a reader can use without
  reading its body.
- Don't split a package only to guard one type's fields. Keep such a type unexported where it is used.
- Guard locked fields with layout, such as a `mu struct{...}` that holds the lock and the fields it guards. Put one
  comment above them, stating the rule they obey: "read and written only while holding sem."
- Use `internal/` to decide who may import a split. It does not supply a reason to split.

## Files

- Give a piece its own file when the rest of the package reaches it only through its funcs, never its fields.
- Confirm the seam in git history: most commits should change the piece or the rest, not both.
- Name each file for its concept. Give one role the same file name everywhere, such as `outbox.go` in every service. A
  file name then works as an index from `ls`.
- Size alone never splits a file.
- For an agent, `go doc -all` reads the package and ignores its files. A split changes only what a source read costs.

## The call site is the API

Judge an API by how its call looks across every use, read without its docs.

```go
// Positional: compiles safely, since the funcs have distinct types, but names no role.
s.pay = aggregate.NewOnce((*model.Order).Paying, s.authorize, s.paid)

// Roles named: the reader needs no hop to learn what each argument does.
s.pay = aggregate.On(orders).Wait((*model.Order).Paying).Send(s.authorize).Keep(s.paid)
```

- Count go-to-definition hops. Four hops to learn what an argument does is too many.
- A doc must land: a reader who jumps to a symbol learns what it does and when, without a second hop. `go doc` groups
  constructors under their type, but go-to-definition never shows that grouping.
- To name roles, pick one: an option struct, a builder chain, or labeled wrapper funcs. Positional funcs fit only when
  the call is obvious.
- In Go 1.27, a generic struct literal must list every type argument. An option struct over generic funcs then costs
  type arguments or an alias. A builder or a constructor infers them.
- A struct of funcs compiles with a nil field. A builder or positional form makes a missing step a compile error.
- Reuse a method name across types only with the same signature and meaning. One word means one thing.
- Avoid a name that already means something in Go. A `Once.Do` that does not mean `sync.Once.Do` is a pun.
- Avoid conjoined methods, which a reader must read together to understand either.
- Avoid shallow pass-throughs, such as a func that only calls another with a default.
- A shared signature can hide a choice. When which of two funcs with one signature a caller passes decides what the
  module does, fold the choice into the module.
- Write programs by writing code, not by defining types. Method expressions such as `(*model.Order).Paying` pass
  behavior without a new type.

## Make misuse impossible

The best rule is one the compiler checks. Prefer, in order: a compile error, a panic at startup, a failing test, and
last, a sentence in a doc.

- Seal an interface with an unexported method, so no other package's type can satisfy it.
- When only the framework may supply a value, give it a type the caller cannot build usefully, such as a reply that
  is valid only while the framework still waits on it.
- A wrong value that the compiler accepts should panic at startup, such as a codec that refuses an untagged field.
- Make every exported name have a caller outside its package that is not a test. If only tests use it, unexport it.
  Packages whose names end in `test` are exempt.
- Write arch tests for the rules the compiler skips: forbidden imports, state changed outside its one method, a
  handler that writes twice.
- List the rest as Traps in the project's `docs/practices.md`: rules that pass the compiler, the tests and the
  linters. Each trap names the doc that holds its fact.

## Inside a function

A reader holds one function at a time. Count what they must hold, and let a tool keep the count.

- Count cognitive complexity, not paths (Campbell). Each break in the flow costs 1, plus 1 for each level it is
  nested. Keep a function at 15 or under.
- A switch with one case for each event or rule is one place to read. Count it as 1 plus its costliest case, never
  as their sum.
- Write a command's refusals as one list of guard cases. Each case is one rule, with its own message.
- Give a value's validity a name, such as `deliverable(a)`. The guard stays in its place, and reads as its rule.
- State a rule once. When a named predicate already holds it, call that predicate rather than restating it negated.
- Keep a condition to one `&&` or `||`. Name the rest only when the name says what the code does not.
- Keep a function to 80 lines of code and 4 parameters. Past 4, name the roles, as the call site section says.
- Extract a helper when its name pays for the hop, never to move a count.
- Read each new name at its call sites, as someone who has not opened the helper. A name fails there when it has no
  subject, asks two questions, hides a step, or reads as a call it does not make.
- In a test, one assertion helper often removes most of the repetition.
- A helper's arguments are all evaluated, unlike the operands of `||`. Never pass a check that another check guards.

### What a score is worth

- Keep a score of your own, built from your rules. `go tool health` is this guide's. It counts a function's
  complexity, length, conditions and parameters, a body that repeats another's, and a pass-through.
- Learn from a vendor's score, then check what it sees. One scored files clean whose worst functions it never
  parsed: each function that takes a func parameter, and each method on a type with two type parameters.
- Probe a scoring tool with a function you know is bad.
- Where a score and a rule disagree, write the rule's reason beside it, and follow the rule.
- A named boolean or a `case` list can clear a finding and leave the logic as complex. That games the score.
- Undo a step that raises the score but reads worse.
- Expect a score to cost lines in code that repeats little.
- When a finding sits in a signature that an interface fixes, redesign the API, not the file.
- A clean score never ends a review. No tool counts hops, or the rules the compiler does not check.

## Errors across a boundary

- Give each error that crosses a package boundary a kind, and let callers compare it with `errors.Is`.
- The package that owns the fact declares the kind, even when another package returns it.
- Pass on another package's refusal unchanged when it is about the request. Name each kind passed on in the doc.
- Reply an unknown error as an opaque `internal error`, so its cause never reaches a client.

## Docs are the contract

- A package doc shows before it explains. Open with a short code sample or a `text` diagram, then add only the rules the
  sample cannot show. Go's `context` package is the model.
- Give each package that others use a runnable Example, which `go test` checks against its Output. One Example
  of the whole module teaches its pattern faster than prose.
- Put each fact a user could learn only from the source in the package doc: a limit, a timeout, a retry interval, a
  name-length cap.
- Keep rules about how packages compose in the project's `docs/practices.md`, not in any one package doc.
- Prose that repeats an example gets cut.
- State each fact in one place. Cut a sentence that another doc states, or that a check enforces with the fix.
- A doc that states a number goes stale when the number moves. Change both in one commit.

### Where a sentence goes

Decide by what to fix when the code disagrees with it.

| The sentence says                          | It goes in                | When code disagrees  |
|--------------------------------------------|---------------------------|----------------------|
| what the code is                           | README or the package doc | fix the doc          |
| a rule a reviewer would reject a commit on | `docs/practices.md`       | fix the code         |
| why one package has its rule               | that package's doc        | fix the doc          |
| a reason no package owns, or a plan        | a decisions log           | revisit the decision |
| something true only because an AI works    | `CLAUDE.md`               | fix the instruction  |
| work still open, or a fact no file holds   | the handoff note          | drop the line        |

Change the README in the same commit as the code it describes. A stale doc is a bug.

### Comments

- A doc comment starts with the name it documents and says what the thing is for.
- Elsewhere, a comment marks a trap, or says why a line that looks wrong is right.
- Say only what the code and its names cannot.
- Write plain, complete sentences, each with one idea and at most 20 words. Enforce the limit with a tool.
- Reword an idea that needs more than 20 words, rather than splitting it in two.
- Put every noun on the page. No pronoun or possessive stands for a thing its sentence does not name.
- Use the word a programmer expects for a mechanism, such as cache, retry or parse.
- A func's doc says what the caller gets back, and when, by the names of its parameters and results.
- An instruction file states what good looks like, and names the file that shows the pattern.

## Tests

- Test the domain rules and the ways the code is wired. Those two are enough.
- Write the rules as one table that reads as the spec: name, given state, command, then the result or error.
- Run each row through the real machinery, not a mock of it, so the test exercises the public API as callers use it.
- Run each wiring once for each transport, such as in process and over HTTP, all in parallel.
- Give each late or repeated reply its own row.
- Fix a bug by first writing its test, and show it failing on the old code.
- Do not repeat a rule's test at a higher layer.
- Prove a refactor by planting a bug in each moved piece. A plant that passes every test names an untested rule.
- Where no test reaches, run the old and the new build on the same inputs, and diff their output.
- A shorter test is no gain if it checks less. Count its calls and its checks before and after.

## Judging two API designs

Score both designs on five weighted measures:

| # | Weight | Measure                                                                        |
|---|-------:|--------------------------------------------------------------------------------|
| 1 |      3 | Fewer go-to-definition hops, summed over every line a reader traces            |
| 2 |      2 | Fewer shallow pass-through functions                                           |
| 3 |      2 | Less code that says nearly the same thing as other code, in any package        |
| 4 |      3 | Fewer files and lines that state or rely on a rule the compiler does not check |
| 5 |      1 | Fewer lines of code                                                            |

- In measure 1, a jump to a package doc counts as a hop, as a jump to a symbol's doc does.
- In measure 4, a line counts once, however many rules it touches, and however a rule is split. It counts twice when
  breaking its rule would store wrong data, or make or skip an outside call.
- For measure 4, and for the measure that decides, the judge cites each file and line it counts.
- Give each measure a margin: 0 when equal, 2 when one count is at least twice the other and they differ by at least 3,
  otherwise 1. For size, under 1% is 0, and 5% or more is 2.
- The winner of a measure scores its weight times its margin. The higher total wins. A tie goes to the first measure
  that differs.
- Size weighs least. Alone, it can pick a design that spreads one rule over several places.
- Report a difference in behavior beside the score, not in it. The requirements settle it.
- Keep the table with the verdict. When the kept design proves wrong, the table shows which weight misled.

## Working with agents on an API

- For a change that shapes a framework, have two agents design apart from the same brief. Leave your favored design
  out of the brief. Each builds in its own worktree until every test passes.
- Where the two converge, keep that. Where they split, a fresh judge scores both without your lean.
- Record the rejected design, and the fact that decided, under the design's Alternatives Considered or in the
  commit message.
- When your lean matches the human's, have a blind agent weigh the options first. Give it the facts, not whose option
  is whose. Hiding whose idea it is reduces sycophancy, while instructions alone do not.
- After a change, run a fresh audit agent that reads whole packages and reports where code breaks a promise. Bring
  each finding to the human, and prefer a design that makes it impossible over a patch.
- Do not audit the fix for an audit finding. Chasing one finding after another hides a better design.
- Before approving a design, run two readers once each. One asks whether each piece is needed now, or already exists.
  The other paraphrases the design as a newcomer and reports where they could not follow it.
- Never contest a comprehension gap. If a fresh reader could not follow the design, the design lacks context.
- Draft a design before its first discussion point. Its open questions, ranked with a lean, form the agenda. A cut
  of scope is a question too.
- Tell an agent to run `go doc -all` on a package before using it.
- Keep one handoff note for the next session. It holds only what is open, and the facts no file holds. Git holds
  what was done.
- Log each reply that confused or slowed the human, with its cause and the correction. Review the log before you
  change an instruction file.

## Mechanical checks

Let tools hold every rule that needs no judgment.

- `gofmt`, plus `go tool style` for blank lines after blocks, a 120-column limit and the 20-word sentence limit.
- Arch tests for import rules and for where state may change.
- A test that fails while `go fix` would change the code.
- A score for each file from `go tool health`, and a test that fails when a file's score moves.
- A test that each code span in the Markdown docs names a real declaration.
- The IDE's full inspection, run from the command line.
