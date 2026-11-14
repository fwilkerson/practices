// Command inspect runs GoLand's inspections over a project, as Code › Inspect Code does. It prints one line for each
// finding. It exits 1 when there is a finding:
//
//	go tool inspect
//
// It inspects the directory it is given, or the working directory, by the profile in that project's .idea. It needs
// GoLand installed, and -app names an install outside /Applications. It runs alongside an open GoLand, on a copy of
// GoLand's settings, so it leaves the IDE's settings alone.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
)

type problem struct {
	File        string `json:"file"`
	Line        int    `json:"line"`
	Description string `json:"description"`
	Class       struct {
		ID string `json:"id"`
	} `json:"problem_class"`
}

func main() {
	app := flag.String("app", "/Applications/GoLand.app", "the GoLand install")
	flag.Parse()
	project, err := filepath.Abs(flag.Arg(0))
	if err != nil {
		log.Fatal(err)
	}

	lines, err := inspect(*app, project)
	if err != nil {
		log.Fatal(err)
	}

	slices.Sort(lines)
	for _, l := range lines {
		fmt.Println(l)
	}

	if len(lines) > 0 {
		os.Exit(1)
	}
}

func inspect(app, project string) ([]string, error) {
	tmp, err := os.MkdirTemp("", "inspect")
	if err != nil {
		return nil, err
	}

	defer func() { _ = os.RemoveAll(tmp) }()
	props, err := isolate(tmp)
	if err != nil {
		return nil, err
	}

	out := filepath.Join(tmp, "out")
	cmd := exec.Command(filepath.Join(app, "Contents/bin/inspect.sh"), project, filepath.Join(project, ".idea"), out,
		"-v0", "-format", "json")
	cmd.Env = append(os.Environ(), "GOLAND_PROPERTIES="+props)
	if b, err := cmd.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("%w\n%s", err, b)
	}

	return report(out, project)
}

// isolate copies GoLand's settings and license into dir. It returns the path of a properties file that points GoLand
// at the copies. A second GoLand refuses to start on the settings and caches that a running GoLand uses.
func isolate(dir string) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	found, _ := filepath.Glob(filepath.Join(home, "Library/Application Support/JetBrains/GoLand*"))
	if len(found) == 0 {
		return "", fmt.Errorf("no GoLand settings under %s", home)
	}

	settings := found[len(found)-1]
	config := filepath.Join(dir, "config")
	if err := os.CopyFS(filepath.Join(config, "options"), os.DirFS(filepath.Join(settings, "options"))); err != nil {
		return "", err
	}

	key, err := os.ReadFile(filepath.Join(settings, "goland.key"))
	if err != nil {
		return "", err
	}

	if err := os.WriteFile(filepath.Join(config, "goland.key"), key, 0o600); err != nil {
		return "", err
	}

	props := filepath.Join(dir, "idea.properties")
	body := fmt.Sprintf("idea.config.path=%s\nidea.system.path=%s\nidea.log.path=%s\nidea.plugins.path=%s\n",
		config, filepath.Join(dir, "system"), filepath.Join(dir, "log"), filepath.Join(settings, "plugins"))
	return props, os.WriteFile(props, []byte(body), 0o600)
}

// report reads GoLand's JSON results in dir, one file per inspection, and returns a line for each finding.
func report(dir, project string) ([]string, error) {
	files, err := filepath.Glob(filepath.Join(dir, "*.json"))
	if err != nil {
		return nil, err
	}

	var lines []string
	for _, f := range files {
		// An aggregate file repeats another inspection's findings in a format of its own.
		if strings.HasSuffix(f, "_aggregate.json") {
			continue
		}

		b, err := os.ReadFile(f)
		if err != nil {
			return nil, err
		}

		var r struct{ Problems []problem }
		if err := json.Unmarshal(b, &r); err != nil {
			return nil, fmt.Errorf("%s: %w", f, err)
		}

		for _, p := range r.Problems {
			path := strings.TrimPrefix(p.File, "file://$PROJECT_DIR$/")
			lines = append(lines, fmt.Sprintf("%s:%d: %s (%s)", filepath.Join(project, path), p.Line, p.Description,
				p.Class.ID))
		}
	}

	return lines, nil
}
