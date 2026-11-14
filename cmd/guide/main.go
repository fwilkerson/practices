// Command guide prints the guide, Writing and Judging a Go API, as Markdown:
//
//	go tool guide
//
// The guide is built into the command. So a project reads the guide of the module version its go.mod names, and
// needs no path to the module.
package main

import (
	"fmt"

	"github.com/fwilkerson/practices"
)

func main() {
	fmt.Print(practices.Guide)
}
