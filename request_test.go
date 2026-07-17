package main

import "testing"

func TestLoadConfigSupportsPostBody(t *testing.T) {
	t.Setenv("KEYWORD", "Jalur 1")
	t.Setenv("TELEGRAM_BOT_TOKEN", "token")
	t.Setenv("TELEGRAM_CHAT_IDS", "-5414949051")
	t.Setenv("WATCH_METHOD", "POST")
	t.Setenv("WATCH_BODY", "0:{}")

	cfg, err := loadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.WatchMethod != "POST" {
		t.Fatalf("WatchMethod = %q, want POST", cfg.WatchMethod)
	}
	if cfg.WatchBody != "0:{}" {
		t.Fatalf("WatchBody = %q, want body", cfg.WatchBody)
	}
}
