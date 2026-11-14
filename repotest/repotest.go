// Package repotest holds the tests that check a whole repository by the guide's "Mechanical checks". A project's
// root test file calls each one it wants:
//
//	func TestStyle(t *testing.T)  { repotest.Style(t) }
//	func TestModern(t *testing.T) { repotest.Modern(t) }
//	func TestGuide(t *testing.T)  { repotest.Guide(t, "docs/guide.md") }
//	func TestNames(t *testing.T)  { repotest.Names(t, "README.md", "docs/practices.md", ".claude/agents/*.md") }
//	func TestHealth(t *testing.T) { repotest.Health(t, held) }
//
//	// held is the score of each file under 10. Each stays until its failures are fixed.
//	var held = map[string]string{"internal/feed/feed.go": "9"}
//
// Each test reads the working directory, which go test sets to the test's own directory. So the test file sits at
// the repository's root. [Style] and [Health] run this module's commands with go tool, so the project's go.mod
// names both in its tool lines.
package repotest

import (
	"os"
	"os/exec"
	"regexp"
	"testing"

	"github.com/fwilkerson/practices"
	"github.com/fwilkerson/practices/claim"
)

// Style fails t while go tool style -l finds a problem in the repository. Running go tool style fixes the formatting
// and the blank lines in place. It only reports a wide line or a long comment sentence, so fix those by hand.
func Style(t testing.TB) {
	t.Helper()
	if out, err := exec.Command("go", "tool", "style", "-l", ".").CombinedOutput(); err != nil {
		t.Errorf("go tool style -l .: %v\n%s", err, out)
	}
}

// Modern fails t while go fix would change the code, such as to use min or range over an int. Running go fix ./...
// applies its changes.
func Modern(t testing.TB) {
	t.Helper()
	out, err := exec.Command("go", "fix", "-diff", "./...").CombinedOutput()
	if err != nil || len(out) > 0 {
		t.Errorf("go fix -diff ./...: %v\n%s", err, out)
	}
}

// Guide fails t while the file at path differs from the guide of the practices module that go.mod names. A project's
// CLAUDE.md loads that file, so each session reads the guide at that version. Running go tool guide > path brings the
// file up to date.
func Guide(t testing.TB, path string) {
	t.Helper()
	got, err := os.ReadFile(path)
	if err != nil || string(got) != practices.Guide {
		t.Errorf("%s differs from the guide, %v: run go tool guide > %s", path, err, path)
	}
}

// Health fails t when a file's score from go tool health differs from its score in held. held maps each file that
// scores under 10 to its score, so a file that held omits must score 10. A failure means the code changed: fix the
// code, and never raise a held score by hand.
func Health(t testing.TB, held map[string]string) {
	t.Helper()
	out, err := exec.Command("go", "tool", "health", ".").CombinedOutput()

	// The command exits 1 while a file scores under 10. Only a run that printed no score could not run.
	got := scores(string(out))
	if err != nil && len(got) == 0 {
		t.Fatalf("go tool health .: %v\n%s", err, out)
	}

	if len(held) == 0 && len(got) == 0 {
		return
	}

	if diff := claim.Diff(held, got); diff != "" {
		t.Errorf("go tool health's scores differ (-held +got):\n%s\n%s", diff, out)
	}
}

// score matches the line go tool health prints for a file under 10: the score, two spaces, then the path.
var score = regexp.MustCompile(`(?m)^ ?(\d+) {2}(\S+)$`)

// scores reads the score of each file under 10 from what go tool health printed.
func scores(out string) map[string]string {
	got := map[string]string{}
	for _, m := range score.FindAllStringSubmatch(out, -1) {
		got[m[2]] = m[1]
	}

	return got
}
