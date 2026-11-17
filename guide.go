// Package practices holds the guide, Writing and Judging a Go API, as Markdown. A project prints the guide with a
// command, at the version of this module that the project's go.mod names:
//
//	$ go tool guide
//	# Writing and Judging a Go API
//
//	The rules a project's Go code is written and judged by. ...
package practices

import _ "embed"

// Guide is guide.md, Writing and Judging a Go API, as Markdown.
//
//go:embed guide.md
var Guide string
