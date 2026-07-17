package main

import "testing"

func TestNormalizeHeaderValueFixesTelegramUnderscoreLoss(t *testing.T) {
	got := normalizeHeaderValue("%22_PAGE_%22")
	if got != "%22__PAGE__%22" {
		t.Fatalf("got %q", got)
	}
}
