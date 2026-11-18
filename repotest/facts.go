package repotest

import (
	"fmt"
	"go/parser"
	"go/token"
	"maps"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/fwilkerson/practices/claim"
)

// Facts fails t for each fact the package doc states that the code no longer holds. facts maps a phrase of the
// doc to whether the code still holds it. One row is "waits 10 s": wait == 10*time.Second. It fails for a phrase
// the doc no longer holds too, so no row outlives its sentence. A package's own test calls it, since go test runs
// that test in the package's directory.
func Facts(t testing.TB, facts map[string]bool) {
	t.Helper()
	problems, err := staleFacts(".", facts)
	claim.Must(t, err)

	for _, p := range problems {
		t.Error(p)
	}
}

// staleFacts returns a problem for each phrase of facts that maps to false, or that dir's package doc lacks. A
// phrase may cross a line break of the doc. A directory with no package doc is an error.
func staleFacts(dir string, facts map[string]bool) ([]string, error) {
	doc, err := packageDoc(dir)
	if err != nil {
		return nil, err
	}

	var problems []string
	for _, phrase := range slices.Sorted(maps.Keys(facts)) {
		switch {
		case !strings.Contains(doc, strings.Join(strings.Fields(phrase), " ")):
			problems = append(problems, fmt.Sprintf("the package doc no longer says %q. State it there, or drop "+
				"its row.", phrase))
		case !facts[phrase]:
			problems = append(problems, fmt.Sprintf("the package doc says %q, which the code no longer holds. "+
				"Change the doc and its row together.", phrase))
		}
	}

	return problems, nil
}

// packageDoc returns the package doc of dir's Go files, test files aside. Each run of space in it is one space.
func packageDoc(dir string) (string, error) {
	files, _ := filepath.Glob(filepath.Join(dir, "*.go"))
	var words []string
	for _, file := range files {
		if strings.HasSuffix(file, "_test.go") {
			continue
		}

		parsed, err := parser.ParseFile(token.NewFileSet(), file, nil, parser.ParseComments|parser.PackageClauseOnly)
		if err != nil {
			return "", err
		}

		words = append(words, strings.Fields(parsed.Doc.Text())...)
	}

	if len(words) == 0 {
		return "", fmt.Errorf("no Go file in %s has a package doc", dir)
	}

	return strings.Join(words, " "), nil
}
