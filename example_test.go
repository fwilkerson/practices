package practices_test

import (
	"fmt"
	"strings"

	"github.com/fwilkerson/practices"
)

func Example() {
	title, _, _ := strings.Cut(practices.Guide, "\n")
	fmt.Println(title)
	// Output:
	// # Writing and Judging a Go API
}
