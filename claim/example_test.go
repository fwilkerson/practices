package claim_test

import (
	"fmt"
	"time"

	"github.com/fwilkerson/practices/claim"
)

func Example() {
	type payment struct {
		Status  string
		Version int
	}
	p := payment{Status: "authorized", Version: 2}

	fmt.Println(claim.Holds(p.Status == "authorized", p.Version == 2))
	fmt.Println(claim.Holds(p.Status == "authorized", p.Version == 3))
	fmt.Println(claim.Diff(payment{Status: "authorized", Version: 3}, p) != "")
	fmt.Println(claim.Diff(p, p) == "")

	// Diff reads stamp's unexported field. It also finds the location, which time.Time's Equal method ignores.
	type stamp struct{ at time.Time }
	noon := time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC)
	fmt.Println(claim.Diff(stamp{noon}, stamp{noon.In(time.FixedZone("EST", -5*60*60))}) != "")
	// Output:
	// true
	// false
	// true
	// true
	// true
}
