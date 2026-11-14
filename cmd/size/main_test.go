package main

import (
	"testing"

	"github.com/fwilkerson/practices/claim"
)

func TestLines(t *testing.T) {
	src := "// Package p\npackage p\n\n/*\nblock\n*/\nvar s = `raw\n\nstring` // trailing\n\n\t// indented\nfunc f() {}"
	got, err := lines([]byte(src))
	claim.Must(t, err)

	want := count{code: 5, comment: 5}
	if got != want {
		t.Errorf("lines() = %+v; want %+v", got, want)
	}
}
