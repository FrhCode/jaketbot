package main

import "testing"

func TestParseCurl(t *testing.T) {
	raw := "curl 'https://jaketboat.bankjakarta.co.id/'\n  -H 'Accept: text/x-component'\n  -H 'next-action: abc'\n  --data-raw '[]'"

	headers, body, ok := parseCurl(raw)
	if !ok {
		t.Fatal("parseCurl failed")
	}
	if headers["Accept"] != "text/x-component" {
		t.Fatalf("Accept = %q", headers["Accept"])
	}
	if headers["next-action"] != "abc" {
		t.Fatalf("next-action = %q", headers["next-action"])
	}
	if body != "[]" {
		t.Fatalf("body = %q", body)
	}
}
