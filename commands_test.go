package main

import (
	"strings"
	"testing"
)

func TestCommandListIncludesControls(t *testing.T) {
	got := commandList()
	for _, want := range []string{"/commands", "/pause", "/start", "/logs", "/curl"} {
		if !strings.Contains(got, want) {
			t.Fatalf("commandList missing %s in %q", want, got)
		}
	}
}

func TestRuntimePauseResume(t *testing.T) {
	r := newRuntimeConfig(Config{})
	if r.isPaused() {
		t.Fatal("new runtime paused")
	}
	r.pause()
	if !r.isPaused() {
		t.Fatal("pause did not pause")
	}
	r.resume()
	if r.isPaused() {
		t.Fatal("resume did not resume")
	}
}
