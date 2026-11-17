// Command guide prints the guide, Writing and Judging a Go API, as Markdown:
//
//	$ go tool guide
//	# Writing and Judging a Go API
//
//	The rules a project's Go code is written and judged by. ...
//
// The guide is built into the command. So a project reads the guide of the module version its go.mod names, and
// needs no path to the module. A project keeps a copy with go tool guide > docs/guide.md, and repotest.Guide fails
// while the copy is stale.
package main

import (
	"fmt"

	"github.com/fwilkerson/practices"
)

func main() {
	fmt.Print(practices.Guide)
}
