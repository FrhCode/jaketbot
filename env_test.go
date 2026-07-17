package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadDotEnv(t *testing.T) {
	t.Setenv("KEYWORD", "")
	t.Setenv("EXISTING", "from-env")

	path := filepath.Join(t.TempDir(), ".env")
	if err := os.WriteFile(path, []byte("KEYWORD=Jakarta\nEMPTY=\n# comment\nSPACED = value with spaces \nEXISTING=from-file\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	loadDotEnv(path)

	if got := os.Getenv("KEYWORD"); got != "" {
		t.Fatalf("KEYWORD = %q, want existing empty env to win", got)
	}
	if got := os.Getenv("EMPTY"); got != "" {
		t.Fatalf("EMPTY = %q, want empty", got)
	}
	if got := os.Getenv("SPACED"); got != "value with spaces" {
		t.Fatalf("SPACED = %q, want value with spaces", got)
	}
	if got := os.Getenv("EXISTING"); got != "from-env" {
		t.Fatalf("EXISTING = %q, want from-env", got)
	}
}
