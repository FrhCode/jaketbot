package main

import "testing"

func TestParseCurlCookie(t *testing.T) {
	got := parseCurlCookie(`-b $'dkib=\u0021abc; TS01=xyz'`)
	if got != "dkib=!abc; TS01=xyz" {
		t.Fatalf("got %q", got)
	}
}
