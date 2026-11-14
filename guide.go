// Package practices holds the guide that the developer's Go projects share. A project reads it with the guide
// command, at the version of this module its go.mod names:
//
//	go tool guide
package practices

import _ "embed"

// Guide is guide.md, Writing and Judging a Go API, as Markdown.
//
//go:embed guide.md
var Guide string
