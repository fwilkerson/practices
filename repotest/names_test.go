package repotest

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/fwilkerson/practices/claim"
)

// TestUnknownNames plants one doc in a scratch repository. Each of its code spans is one that Names allows, but for
// the two on its last lines.
func TestUnknownNames(t *testing.T) {
	root := t.TempDir()
	write(t, root, "order/order.go", "package order\n\ntype Order struct{ Total int }\n\nfunc (o *Order) Pay() {}\n")
	write(t, root, "order/order_test.go", "package order_test\n\nfunc TestPay() {}\n")
	write(t, root, "order/testdata/skipped.go", "not Go, and never read")
	write(t, root, "docs/order.md", `# Order

An `+"`Order`"+` holds its `+"`Order.Total`"+`, and `+"`order.Order.Pay`"+` pays it, as `+"`TestPay`"+` shows.
A lower-case word such as `+"`order`"+`, a file such as `+"`README.md`"+` and `+"`HTTP`"+` are no names.
So is `+"`http.Get`"+`, from a package outside the repository.
But `+"`Order.Refund`"+` names nothing.
Neither does `+"`order.Missing`"+`, though its package exists.
`)

	got, err := unknownNames(root, []string{"docs/*.md"})
	claim.Must(t, err)

	want := []string{
		"docs/order.md:6: `Order.Refund` names nothing in this repository. Fix it, or write it in words.",
		"docs/order.md:7: `order.Missing` names nothing in this repository. Fix it, or write it in words.",
	}
	if diff := claim.Diff(want, got); diff != "" {
		t.Errorf("unknownNames' problems differ (-want +got):\n%s", diff)
	}
}

// TestUnknownNamesNoDoc holds that a pattern that matches no file is an error, so a misspelled path never passes.
func TestUnknownNamesNoDoc(t *testing.T) {
	if got, err := unknownNames(t.TempDir(), []string{"README.md"}); err == nil {
		t.Errorf("unknownNames = %q, nil; want an error for a doc that does not exist", got)
	}
}

// TestScores reads what go tool health prints: each failure, then the score of each file under 10.
func TestScores(t *testing.T) {
	out := "a/b.go:12: f's cognitive complexity is 17\n 9  a/b.go\n 7  c.go\n"
	want := map[string]string{"a/b.go": "9", "c.go": "7"}
	if diff := claim.Diff(want, scores(out)); diff != "" {
		t.Errorf("scores differ (-want +got):\n%s", diff)
	}
}

// write writes body to the file at path under root, and makes the directories above it.
func write(t *testing.T, root, path, body string) {
	t.Helper()
	path = filepath.Join(root, path)
	claim.Must(t, os.MkdirAll(filepath.Dir(path), 0o755))
	claim.Must(t, os.WriteFile(path, []byte(body), 0o644))
}
