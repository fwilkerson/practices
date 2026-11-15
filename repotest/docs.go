package repotest

import (
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/fwilkerson/practices/claim"
)

// Links fails t for each link in docs whose target is a path that names no file. Each of docs is a path or a glob
// of Markdown files, as [Names] takes them. Links skips a URL, an anchor within the page and a fenced code block. A
// path that starts with a slash is read from the repository's root.
func Links(t testing.TB, docs ...string) {
	t.Helper()
	problems, err := brokenLinks(".", docs)
	claim.Must(t, err)

	for _, p := range problems {
		t.Error(p)
	}
}

// Context fails t while a file that every session loads grows past its limit, or imports another file. limits maps
// each such file, such as CLAUDE.md, to the most words it may hold.
func Context(t testing.TB, limits map[string]int) {
	t.Helper()
	problems, err := heavyContext(".", limits)
	claim.Must(t, err)

	for _, p := range problems {
		t.Error(p)
	}
}

// link is a Markdown link's target, up to its closing parenthesis or its title.
var link = regexp.MustCompile(`\]\(([^)\s]+)`)

// brokenLinks returns a problem for each link, in the docs under root, whose path names no file. A pattern of docs
// that matches no file is an error, so a misspelled path never passes.
func brokenLinks(root string, docs []string) ([]string, error) {
	var problems []string
	for _, pattern := range docs {
		found, _ := filepath.Glob(filepath.Join(root, pattern))
		if len(found) == 0 {
			return nil, fmt.Errorf("%s matches no file under %s", pattern, root)
		}

		for _, doc := range found {
			lines, err := prose(doc)
			if err != nil {
				return nil, err
			}

			problems = append(problems, brokenIn(root, doc, lines)...)
		}
	}

	return problems, nil
}

// brokenIn returns a problem for each link in lines whose target names no file. lines is the prose of the doc at
// path.
func brokenIn(root, path string, lines map[int]string) []string {
	doc, _ := filepath.Rel(root, path)

	var problems []string
	for _, n := range slices.Sorted(maps.Keys(lines)) {
		for _, m := range link.FindAllStringSubmatch(lines[n], -1) {
			target, _, _ := strings.Cut(m[1], "#")
			if target == "" || strings.Contains(target, ":") {
				continue
			}

			file := filepath.Join(filepath.Dir(path), target)
			if strings.HasPrefix(target, "/") {
				file = filepath.Join(root, target)
			}

			if _, err := os.Stat(file); err != nil {
				problems = append(problems, fmt.Sprintf(
					"%s:%d: the link to %s names no file. Fix the path, or drop the link.", doc, n, m[1]))
			}
		}
	}

	return problems
}

// imported is an @ that starts a word, as Claude Code writes the import of another file.
var imported = regexp.MustCompile("(^|\\s)@[~./\\w]")

// codeSpans matches each code span of a line, which Claude Code never reads as an import.
var codeSpans = regexp.MustCompile("`[^`]*`")

// heavyContext returns a problem for each file of limits, under root, that imports a file or holds too many words.
func heavyContext(root string, limits map[string]int) ([]string, error) {
	var problems []string
	for _, path := range slices.Sorted(maps.Keys(limits)) {
		b, err := os.ReadFile(filepath.Join(root, path))
		if err != nil {
			return nil, err
		}

		lines, err := prose(filepath.Join(root, path))
		if err != nil {
			return nil, err
		}

		for _, n := range slices.Sorted(maps.Keys(lines)) {
			if imported.MatchString(codeSpans.ReplaceAllString(lines[n], "")) {
				problems = append(problems, fmt.Sprintf(
					"%s:%d: the line imports a file into every session. Name the file in words, for the agent that "+
						"needs it to read.", path, n))
			}
		}

		if words := len(strings.Fields(string(b))); words > limits[path] {
			problems = append(problems, fmt.Sprintf(
				"%s holds %d words, over %d. Every session loads each one, so move what one agent needs into that "+
					"agent's file. Only the repository's owner raises the limit.", path, words, limits[path]))
		}
	}

	return problems, nil
}

// prose returns the lines of the Markdown file at path that sit outside a fenced code block, by line number.
func prose(path string) (map[int]string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	lines, fenced := map[int]string{}, false
	for i, line := range strings.Split(string(b), "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "```") {
			fenced = !fenced
			continue
		}

		if !fenced {
			lines[i+1] = line
		}
	}

	return lines, nil
}
