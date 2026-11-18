package repotest

import (
	"testing"

	"github.com/fwilkerson/practices/claim"
)

// TestStaleFacts plants a package whose doc states two numbers, over two lines. One row holds, one names a number
// the code moved, and one names a phrase the doc lacks.
func TestStaleFacts(t *testing.T) {
	root := t.TempDir()
	write(t, root, "feed.go", "// Package feed reads a feed. It waits 10 s for a reply, and keeps the last\n"+
		"// 64 KiB of each.\npackage feed\n")
	write(t, root, "feed_test.go", "// Package feed says it retries 3 times.\npackage feed\n")

	got, err := staleFacts(root, map[string]bool{
		"waits 10 s for a reply": true,
		"keeps the last 64 KiB":  false,
		"retries 3 times":        true,
	})
	claim.Must(t, err)

	want := []string{
		`the package doc says "keeps the last 64 KiB", which the code no longer holds. Change the doc and its row ` +
			`together.`,
		`the package doc no longer says "retries 3 times". State it there, or drop its row.`,
	}
	if diff := claim.Diff(want, got); diff != "" {
		t.Errorf("staleFacts' problems differ (-want +got):\n%s", diff)
	}
}

// TestStaleFactsNoDoc holds that a directory with no package doc is an error. So a test in the wrong place never
// passes.
func TestStaleFactsNoDoc(t *testing.T) {
	if got, err := staleFacts(t.TempDir(), map[string]bool{"waits 10 s": true}); err == nil {
		t.Errorf("staleFacts = %q, nil; want an error for a directory with no package doc", got)
	}
}
