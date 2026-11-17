package repotest

import (
	"testing"

	"github.com/fwilkerson/practices/claim"
)

// TestBrokenLinks plants one doc in a scratch repository. Each of its links is one that Links allows, but for the
// two on its last lines.
func TestBrokenLinks(t *testing.T) {
	root := t.TempDir()
	write(t, root, "README.md", "# Root\n")
	write(t, root, "docs/guide.md", "# Guide\n")
	write(t, root, "docs/order.md", `# Order

See [the guide](guide.md), [its rules](guide.md#rules), [the root](../README.md) and [the root again](/README.md).
A [URL](https://go.dev/doc), [mail](mailto:dev@example.com) and [an anchor](#order) are no paths.
`+"```text\n[a link in a code block](missing.md)\n```"+`
But [the plan](../notes/plan.md) names no file.
Neither does [a section](gone.md#rules).
`)

	got, err := brokenLinks(root, []string{"docs/order.md"})
	claim.Must(t, err)

	want := []string{
		"docs/order.md:8: the link to ../notes/plan.md names no file. Fix the path, or drop the link.",
		"docs/order.md:9: the link to gone.md#rules names no file. Fix the path, or drop the link.",
	}
	if diff := claim.Diff(want, got); diff != "" {
		t.Errorf("brokenLinks' problems differ (-want +got):\n%s", diff)
	}
}

// TestBrokenLinksNoDoc holds that a pattern that matches no file is an error, so a misspelled path never passes.
func TestBrokenLinksNoDoc(t *testing.T) {
	if got, err := brokenLinks(t.TempDir(), []string{"README.md"}); err == nil {
		t.Errorf("brokenLinks = %q, nil; want an error for a doc that does not exist", got)
	}
}

// TestHeavyContext plants three instruction files. One is within its limit, one imports a file, and one is too long.
func TestHeavyContext(t *testing.T) {
	root := t.TempDir()
	write(t, root, "ok.md", "# Shop\n\nRun `@docs/guide.md` never loads, and dev@example.com is no import.\n")
	write(t, root, "imports.md", "# Shop\n\n@docs/guide.md\n\n"+"```text\n@a/code/block.md\n```\n")
	write(t, root, "long.md", "one two three four five six\n")

	got, err := heavyContext(root, map[string]int{"ok.md": 20, "imports.md": 20, "long.md": 5})
	claim.Must(t, err)

	want := []string{
		"imports.md:3: the line imports a file into every session. Name the file in words, for the agent that needs " +
			"it to read.",
		"long.md holds 6 words, over 5. Every session loads each one, so move what one agent needs into that agent's " +
			"file. Only the repository's owner raises the limit.",
	}
	if diff := claim.Diff(want, got); diff != "" {
		t.Errorf("heavyContext's problems differ (-want +got):\n%s", diff)
	}
}
