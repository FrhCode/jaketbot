package main

import "testing"

func TestLogBufferTail(t *testing.T) {
	b := newLogBuffer(3)
	b.add("one")
	b.add("two")
	b.add("three")
	b.add("four")

	got := b.tail(10)
	want := []string{"two", "three", "four"}
	if len(got) != len(want) {
		t.Fatalf("len = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("tail[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}
