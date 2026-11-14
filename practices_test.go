// Package practices_test holds this repository to the rules its own commands check.
package practices_test

import (
	"os/exec"
	"strings"
	"testing"
)

// TestRules fails while style or health finds a problem in this repository. The failure shows what the command found.
func TestRules(t *testing.T) {
	for _, command := range []string{"go tool style -l .", "go tool health ."} {
		args := strings.Fields(command)
		if out, err := exec.Command(args[0], args[1:]...).CombinedOutput(); err != nil {
			t.Errorf("%s: %v\n%s", command, err, out)
		}
	}
}
