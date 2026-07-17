package main

import "testing"

func TestDefaultWatchRequestUsesKnownNextHeaders(t *testing.T) {
	t.Setenv("KEYWORD", "Jalur 1")
	t.Setenv("TELEGRAM_BOT_TOKEN", "token")
	t.Setenv("TELEGRAM_CHAT_IDS", "-5414949051")

	cfg, err := loadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.WatchMethod != "POST" {
		t.Fatalf("WatchMethod = %q, want POST", cfg.WatchMethod)
	}
	if cfg.WatchBody != "[]" {
		t.Fatalf("WatchBody = %q, want []", cfg.WatchBody)
	}
	if cfg.WatchHeaders.Get("next-action") == "" {
		t.Fatal("next-action header missing")
	}
}
