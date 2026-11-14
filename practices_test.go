// Package practices_test holds this repository to the rules its own commands check.
package practices_test

import (
	"testing"

	"github.com/fwilkerson/practices/repotest"
)

func TestStyle(t *testing.T)  { repotest.Style(t) }
func TestModern(t *testing.T) { repotest.Modern(t) }
func TestGuide(t *testing.T)  { repotest.Guide(t, "guide.md") }
func TestNames(t *testing.T)  { repotest.Names(t, "README.md") }
func TestHealth(t *testing.T) { repotest.Health(t, nil) }
