// Package claim holds the checks that a project's tests share. Only tests and their helpers import it. A test of a
// payment service uses it this way:
//
//	_, errAuthorize := payments.Authorize(ctx, authorize)
//	_, errCapture := payments.Capture(ctx, capture)
//	claim.Must(t, errAuthorize, errCapture)
//
//	p, err := payments.Get(ctx, ref)
//	if err != nil || !claim.Holds(p.Status == "captured", p.Version == 4) {
//		t.Errorf("Get = %+v, %v; want captured at version 4", p, err)
//	}
//
//	if diff := claim.Diff(want, got); diff != "" {
//		t.Errorf("Feed's events differ (-want +got):\n%s", diff)
//	}
//
// Holds and Diff return what they found, and the test writes the failure in Go's got/want form. The exception is
// [Must], which stops the test with the setup error as its message.
//
// Check a call's error before Holds, with ||, as the sample does. Then the reply is read only after the call succeeds.
// GoLand's check for an unchecked error cannot see inside Holds. So err == nil passed to Holds may draw its warning.
package claim

import (
	"errors"
	"fmt"
	"reflect"
	"slices"
	"testing"
	"uuid"

	"github.com/google/go-cmp/cmp"
)

// Holds reports whether every condition is true. Go evaluates every condition before the call, so one condition cannot
// guard another. An index expression still panics when the length check beside it is false.
func Holds(conditions ...bool) bool {
	return !slices.Contains(conditions, false)
}

// Must stops the test if any of errs is not nil, and reports each one.
func Must(t testing.TB, errs ...error) {
	t.Helper()
	if err := errors.Join(errs...); err != nil {
		t.Fatal(err)
	}
}

// Diff returns a text that shows how got differs from want, or "" when [reflect.DeepEqual] reports them equal. go-cmp's
// cmp.Diff writes the text: a line from want starts with -, and a line from got with +. cmp.Diff alone reports two
// values equal when their type's Equal method does, and time.Time's Equal ignores the location. Diff then returns both
// values in full instead. A [uuid.UUID] prints as its string. cmp.Diff varies its spacing on purpose, so no test may
// compare the text.
func Diff[T any](want, got T) string {
	if reflect.DeepEqual(want, got) {
		return ""
	}

	// cmp.Diff panics on an unexported field unless an Exporter allows it.
	unexported := cmp.Exporter(func(reflect.Type) bool { return true })
	if diff := cmp.Diff(want, got, unexported, cmp.Transformer("String", uuid.UUID.String)); diff != "" {
		return diff
	}

	// cmp found no difference, since an Equal method hid it.
	return fmt.Sprintf("-%+v\n+%+v", want, got)
}
