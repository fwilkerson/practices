package repotest_test

import (
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"testing"

	"github.com/fwilkerson/practices/repotest"
)

// report is a testing.TB that prints each failure, so the Example shows what a test would report.
type report struct{ testing.TB }

func (report) Helper()           {}
func (report) Error(args ...any) { fmt.Println(args...) }
func (report) Fatal(args ...any) { fmt.Println(args...) }

// Example checks a scratch repository whose README names a method that order.go never declares, and links to a
// missing file.
func Example() {
	repo, err := os.MkdirTemp("", "repo")
	if err != nil {
		log.Fatal(err)
	}

	defer func() { _ = os.RemoveAll(repo) }()
	back, err := os.Getwd()
	if err != nil {
		log.Fatal(err)
	}

	code := "package order\n\ntype Order struct{ Total int }\n"
	readme := "# Order\n\nAn `Order` holds its `Order.Total`, and `Order.Refund` pays it back.\n" +
		"[The guide](docs/guide.md) says how.\n"
	err = errors.Join(
		os.WriteFile(filepath.Join(repo, "order.go"), []byte(code), 0o644),
		os.WriteFile(filepath.Join(repo, "README.md"), []byte(readme), 0o644),
		os.Chdir(repo),
	)
	if err != nil {
		log.Fatal(err)
	}

	// Each check reads the working directory, which go test sets to the root test's directory.
	defer func() { _ = os.Chdir(back) }()
	repotest.Names(report{}, "README.md")
	repotest.Links(report{}, "README.md")
	// Output:
	// README.md:3: `Order.Refund` names nothing in this repository. Fix it, or write it in words.
	// README.md:4: the link to docs/guide.md names no file. Fix the path, or drop the link.
}
