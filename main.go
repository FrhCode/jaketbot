package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	WatchURL            string
	Keyword             string
	CheckInterval       time.Duration
	TelegramBotToken    string
	TelegramChatIDs     []string
	StateFile           string
	HTTPTimeout         time.Duration
	UserAgent           string
}

type State struct {
	Found     bool      `json:"found"`
	CheckedAt time.Time `json:"checked_at"`
}

func main() {
	cfg, err := loadConfig()
	if err != nil {
		log.Fatal(err)
	}

	state := loadState(cfg.StateFile)
	client := &http.Client{Timeout: cfg.HTTPTimeout}
	ticker := time.NewTicker(cfg.CheckInterval)
	defer ticker.Stop()

	check := func() {
		now := time.Now().UTC()
		found, err := checkKeyword(client, cfg.WatchURL, cfg.Keyword, cfg.UserAgent)
		if err != nil {
			log.Printf("time=%s url=%s keyword_found=%t error=%v", now.Format(time.RFC3339), cfg.WatchURL, false, err)
			return
		}

		log.Printf("time=%s url=%s keyword_found=%t", now.Format(time.RFC3339), cfg.WatchURL, found)

		if found && !state.Found {
			if err := sendTelegramAlerts(client, cfg, now); err != nil {
				log.Printf("time=%s url=%s keyword_found=%t error=%v", now.Format(time.RFC3339), cfg.WatchURL, found, err)
			}
		}

		state.Found = found
		state.CheckedAt = now
		if err := saveState(cfg.StateFile, state); err != nil {
			log.Printf("time=%s url=%s keyword_found=%t error=%v", now.Format(time.RFC3339), cfg.WatchURL, found, err)
		}
	}

	check()
	for range ticker.C {
		check()
	}
}

func loadConfig() (Config, error) {
	watchURL := getenv("WATCH_URL", "https://jaketboat.bankjakarta.co.id/")
	keyword := strings.TrimSpace(os.Getenv("KEYWORD"))
	token := strings.TrimSpace(os.Getenv("TELEGRAM_BOT_TOKEN"))
	chatIDsRaw := strings.TrimSpace(os.Getenv("TELEGRAM_CHAT_IDS"))
	stateFile := getenv("STATE_FILE", "/data/state.json")
	ua := getenv("USER_AGENT", "Mozilla/5.0 WebsiteKeywordWatcher/1.0")

	intervalSeconds, err := parsePositiveInt(getenv("CHECK_INTERVAL_SECONDS", "300"), "CHECK_INTERVAL_SECONDS")
	if err != nil {
		return Config{}, err
	}
	timeoutSeconds, err := parsePositiveInt(getenv("HTTP_TIMEOUT_SECONDS", "20"), "HTTP_TIMEOUT_SECONDS")
	if err != nil {
		return Config{}, err
	}
	if keyword == "" {
		return Config{}, errors.New("KEYWORD is required")
	}
	if token == "" {
		return Config{}, errors.New("TELEGRAM_BOT_TOKEN is required")
	}
	if chatIDsRaw == "" {
		return Config{}, errors.New("TELEGRAM_CHAT_IDS is required")
	}
	chatIDs := splitCSV(chatIDsRaw)
	if len(chatIDs) == 0 {
		return Config{}, errors.New("TELEGRAM_CHAT_IDS is required")
	}

	return Config{
		WatchURL:         watchURL,
		Keyword:          keyword,
		CheckInterval:    time.Duration(intervalSeconds) * time.Second,
		TelegramBotToken: token,
		TelegramChatIDs:  chatIDs,
		StateFile:        stateFile,
		HTTPTimeout:      time.Duration(timeoutSeconds) * time.Second,
		UserAgent:        ua,
	}, nil
}

func getenv(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}

func parsePositiveInt(v, name string) (int, error) {
	n, err := strconv.Atoi(strings.TrimSpace(v))
	if err != nil || n <= 0 {
		return 0, fmt.Errorf("%s must be positive integer", name)
	}
	return n, nil
}

func splitCSV(v string) []string {
	parts := strings.Split(v, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

func containsKeyword(body, keyword string) bool {
	return strings.Contains(strings.ToLower(body), strings.ToLower(keyword))
}

func checkKeyword(client *http.Client, watchURL, keyword, userAgent string) (bool, error) {
	req, err := http.NewRequest(http.MethodGet, watchURL, nil)
	if err != nil {
		return false, err
	}
	req.Header.Set("User-Agent", userAgent)

	resp, err := client.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return false, fmt.Errorf("unexpected http status %s", resp.Status)
	}

	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return false, err
	}
	return containsKeyword(string(b), keyword), nil
}

func loadState(path string) State {
	b, err := os.ReadFile(path)
	if err != nil {
		return State{}
	}
	var s State
	if err := json.Unmarshal(b, &s); err != nil {
		return State{}
	}
	return s
}

func saveState(path string, state State) error {
	if err := os.MkdirAll(dirOf(path), 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(b, '\n'), 0o644)
}

func dirOf(path string) string {
	i := strings.LastIndex(path, "/")
	if i < 0 {
		return "."
	}
	if i == 0 {
		return "/"
	}
	return path[:i]
}

func sendTelegramAlerts(client *http.Client, cfg Config, now time.Time) error {
	for _, chatID := range cfg.TelegramChatIDs {
		if err := sendTelegramMessage(client, cfg.TelegramBotToken, chatID, cfg.WatchURL, cfg.Keyword, now); err != nil {
			return err
		}
	}
	return nil
}

func sendTelegramMessage(client *http.Client, token, chatID, watchURL, keyword string, now time.Time) error {
	endpoint := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", token)
	message := fmt.Sprintf("Keyword ditemukan\nURL: %s\nKeyword: %s\nTime: %s", watchURL, keyword, now.Format(time.RFC3339))
	form := url.Values{}
	form.Set("chat_id", chatID)
	form.Set("text", message)

	req, err := http.NewRequest(http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return fmt.Errorf("telegram send failed: %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}
	return nil
}

// ponytail: state is single-file JSON for MVP; upgrade to DB only if multi-instance or audit trail needed.
