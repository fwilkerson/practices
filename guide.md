# Writing and Judging a Go API

Lessons distilled from odh, a Go modular monolith built with AI agents. Each rule here was either backed by research
or learned from a mistake. Where a source or a run settled it, the source is named.

## Principles

- Keep the code small and simple. Measure small in lines. Simple is not the same as easy.
- Make an impossible state impossible to build, rather than checking for it at runtime.
- One small reusable piece beats the same glue repeated in each caller.
- Prefer a deep module: a small interface over a lot of behavior (Ousterhout).
- Don't make the client do anything the module could do (Bloch). When in doubt, leave it out.
- Minimize conceptual weight, not the count of types or methods (Bloch).
- Build what the current step needs. Expect names to change and refactors to follow.
- Use modern Go where it removes code: generics, `http.ServeMux` patterns, `log/slog`, `errors.AsType`,
  `testing/synctest`, `sync.WaitGroup.Go`. A test that fails while `go fix` would change the code keeps it modern.

## Packages

A package is a contract its users read through `go doc`, or a piece two packages share.

- Default to fewer packages. Each split adds an import, a package doc, and an exported name for each shared thing.
  Cheney: "Your default position should be to not create a new package."
- Split a package when a second importer needs the code, or for a subsystem whose contract a reader can use without
  reading its body. The standard library moved `singleflight` out only once `cmd/go` needed it too.
- Don't split a package only to guard one type's fields. The standard library and large projects keep such types
  unexported where they are used: `sync.poolDequeue`, `database/sql.closingMutex`, CockroachDB's `mu struct{...}`.
- Guard locked fields with layout and one comment above them, stating the rule they obey: "read and written only while
  holding sem."
- Use `internal/` to decide who may import a split. It does not supply a reason to split.

## Files

- Give a piece its own file when the rest of the package reaches it only through its funcs, never its fields.
- Confirm the seam in git history: most commits should change the piece or the rest, not both.
- Name each file for its concept. Give one role the same file name everywhere, such as `outbox.go` in every service. A
  file name then works as an index from `ls`.
- Size alone never splits a file. Nine Go files in ten stay under 500 lines, and `context` is one file of 807 lines.
- For an agent, `go doc -all` reads the package and ignores its files. A split changes only what a source read costs.

## The call site is the API

Judge an API by how its call looks across every use, read without its docs (Bloch, Google's Go style, Abseil Tip #94).

```go
// Positional: compiles safely, since the funcs have distinct types, but names no role.
s.pay = aggregate.NewOnce((*model.Order).Paying, s.authorize, s.paid)

// Roles named: the reader needs no hop to learn what each argument does.
s.pay = aggregate.On(orders).Wait((*model.Order).Paying).Send(s.authorize).Keep(s.paid)
```

- Count go-to-definition hops. A reader who needs four hops to learn what an argument does has hit an unknown unknown.
- A doc must land: a reader who jumps to a symbol learns what it does and when, without a second hop. `go doc` groups
  constructors under their type, but go-to-definition never shows that grouping.
- To name roles, pick one: an option struct, a builder chain, or labeled wrapper funcs. Positional funcs fit only when
  the call is obvious.
- In Go 1.27, a generic struct literal must list every type argument. An option struct over generic funcs then costs
  type arguments or an alias. A builder or a constructor infers them.
- A struct of funcs compiles with a nil field. A builder or positional form makes a missing step a compile error.
- Reuse a method name across types only with the same signature and meaning (Effective Go). One word means one thing.
- Avoid a name that already means something in Go. A `Once.Do` that does not mean `sync.Once.Do` is a pun.
- Avoid conjoined methods, which a reader must read together to understand either.
- Avoid shallow pass-throughs, such as a func that only calls another with a default.
- A shared signature can hide a choice. When the caller's choice of `Do` decides what gets sent, fold the choice into
  the module.
- Write programs by writing code, not by defining types (Ian Lance Taylor). Method expressions such as
  `(*model.Order).Paying` pass behavior without a new type.

## Make misuse impossible

The best rule is one the compiler checks. Prefer, in order: a compile error, a panic at startup, a failing test, and
last, a sentence in a doc.

- Seal an interface with an unexported method, so no other package's type can satisfy it.
- When only the framework may supply a value, give it a type the caller cannot build usefully. odh's `aggregate.Inbox`
  holds a reply only while the root still waits on it.
- A wrong value that the compiler accepts should panic at startup, such as a codec that refuses an untagged field.
- Make every exported name have a caller outside its package that is not a test. If only tests use it, unexport it.
  Packages whose names end in `test` are exempt.
- Write arch tests for the rules the compiler skips: forbidden imports, state changed outside its one method, a
  handler that writes twice.
- List the rest as Traps: rules that pass the compiler, the tests and the linters. Each trap names the doc that holds
  its fact.

## Inside a function

A reader holds one function at a time. Count what they must hold, and let a tool keep the count.

- Count cognitive complexity, not paths (Campbell). Each break in the flow costs 1, plus 1 for each level it is
  nested. Keep a function at 15 or under.
- A switch with one case for each event or rule is one place to read. Count it as its costliest case, never as their
  sum. odh's `Apply` has 24 cases, and a reader takes each alone.
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

- Keep a score of your own, built from your rules. odh's is six counts, and a test fails when a file's score moves.
- Learn from a vendor's score, then check what it sees. CodeScene gave 10.0 to files whose worst functions it never
  parsed. It skipped each function that takes a func parameter, and each method on a type with two type parameters.
- Probe a scoring tool with a function you know is bad.
- Where a score and a rule disagree, write the rule's reason beside it, and follow the rule.
- A named boolean or a `case` list can clear a finding and leave the logic as complex. That games the score.
- Undo a step that raises the score but reads worse.
- Expect a score to cost lines in code that repeats little. odh's runtime code grew 4% to reach 10.0.
- When a finding sits in a signature that an interface fixes, redesign the API, not the file. odh's `Append` now takes
  one `Write`.
- A clean score never ends a review. No tool counts hops, or the rules the compiler does not check.

## Errors across a boundary

- Give each error that crosses a package boundary a kind, and let callers compare it with `errors.Is`.
- The package that owns the fact declares the kind, even when another package returns it.
- Pass on another package's refusal unchanged when it is about the request. Name each kind passed on in the doc.
- Reply an unknown error as an opaque `internal error`, so its cause never reaches a client.

## Docs are the contract

- A package doc shows before it explains. Open with a short code sample or a `text` diagram, then add only the rules the
  sample cannot show. Go's `context` package is the model.
- Give each package that others use a runnable Example, which `go test` checks against its Output. One whole-module
  Example teaches the pattern faster than prose.
- Put each fact a user could learn only from the source in the package doc: a limit, a timeout, a retry interval, a
  name-length cap. In odh's quiz, a fresh agent scored 16 of 24 on such facts before they were documented, and 24 of 24
  after.
- Keep rules about how packages compose in a practices doc, not in any one package doc. Without it, agents missed
  only those.
- Prose that repeats an example gets cut.
- State each fact in one place. Cut a sentence that another doc states, or that a check enforces with the fix.
- A doc that states a number goes stale when the number moves. Change both in one commit.

### Where a sentence goes

Decide by what to fix when the code disagrees with it.

| The sentence says                          | It goes in                | When code disagrees  |
|--------------------------------------------|---------------------------|----------------------|
| what the code is                           | README or the package doc | fix the doc          |
| a rule a reviewer would reject a commit on | the practices doc         | fix the code         |
| why one package has its rule               | that package's doc        | fix the doc          |
| a reason no package owns, or a plan        | a decisions log           | revisit the decision |
| something true only because an AI works    | `CLAUDE.md`               | fix the instruction  |

Change the README in the same commit as the code it describes. A stale doc is a bug.

### Comments

- A doc comment starts with the name it documents and says what the thing is for.
- Elsewhere, a comment marks a trap, or says why a line that looks wrong is right.
- Say only what the code and its names cannot. Fewer comments go stale less.
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
- Run each wiring once for each transport, such as in process, over HTTP and sharded, all in parallel.
- Give each late or repeated reply its own row.
- Fix a bug by first writing its test, and show it failing on the old code. A failing test ends a debate that prose
  cannot.
- Do not repeat a rule's test at a higher layer.
- Prove a refactor by planting a bug in each moved piece. A plant that passes every test names an untested rule.
  odh's sweep found untested rules in four modules and four framework packages this way.
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
- Size weighs least. In odh, size alone once picked a design that spread one lock rule over three groups of fields.
- Report a difference in behavior beside the score, not in it. The requirements settle it.
- Keep the table with the verdict. When the kept design proves wrong, the table shows which weight misled.

## Working with agents on an API

- For a change that shapes a framework, have two agents design apart from the same brief. Leave your favored design
  out of the brief. Each builds in its own worktree until every test passes.
- Where the two converge, keep that. Where they split, a fresh judge scores both without your lean.
- Record the rejected design, and the fact that decided, under Alternatives Considered or in the commit message.
- When your lean matches the human's, have a blind agent weigh the options first. Give it the facts, not whose option
  is whose. Hiding whose idea it is reduces sycophancy, while instructions alone do not.
- After a change, run a fresh audit agent that reads whole packages and reports where code breaks a promise. Bring
  each finding to the human, and prefer a design that makes it impossible over a patch.
- Do not audit the fix for an audit finding. Chasing one finding after another hides a better design.
- Before approving a design, run two readers once each. One asks whether each piece is needed now, or already exists.
  The other paraphrases the design as a newcomer and reports where they could not follow it.
- Never contest a comprehension gap. If a fresh reader could not follow the design, the design lacks context.
- Draft the document before the first discussion point. Its open questions, ranked with a lean, form the agenda. A
  cut of scope is a question too.
- Tell an agent to run `go doc -all` on a package before using it. Its doc is the contract, and its Example shows the
  use.

## Mechanical checks

Let tools hold every rule that needs no judgment, so a review spends its attention on judgment.

- `gofmt`, plus a style tool for blank lines after blocks, a 120-column limit and the 20-word sentence limit.
- Arch tests for import rules and for where state may change.
- A test that fails while `go fix` would change the code.
- A score for each file from the counts in Inside a function, and a test that fails when a file's score moves.
- A test that each code span in the Markdown docs names a real declaration, so the docs cannot drift from the code.
- The IDE's full inspection run from the command line, since partial linters report only part of it.

## Sources

Ousterhout, *A Philosophy of Software Design* and CS 190. Bloch, "How to Design a Good API". Google's Go Style Guide,
Best Practices. Effective Go. Abseil Tip #94. Cheney, *Practical Go*. Taylor, "When To Use Generics". Cox, "Go 1.4
Internal Packages". Ottinger's naming rules. Sharma et al. 2023 and ELEPHANT 2025 on sycophancy. Campbell, "Cognitive
Complexity" (SonarSource). CodeScene's Code Health.
