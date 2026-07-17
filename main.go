package main

import (
	"bufio"
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
	WatchURL         string
	WatchMethod      string
	WatchBody        string
	WatchContentType string
	WatchHeaders     http.Header
	Keyword          string
	CheckInterval    time.Duration
	TelegramBotToken string
	TelegramChatIDs  []string
	StateFile        string
	HTTPTimeout      time.Duration
	UserAgent        string
}

const (
	defaultWatchMethod = http.MethodPost
	defaultWatchBody   = "[]"
)

type State struct {
	Found          bool      `json:"found"`
	FailureAlerted bool      `json:"failure_alerted"`
	CheckedAt      time.Time `json:"checked_at"`
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
		found, err := checkKeyword(client, cfg)
		if err != nil {
			log.Printf("time=%s url=%s keyword_found=%t error=%v", now.Format(time.RFC3339), cfg.WatchURL, false, err)
			if !state.FailureAlerted {
				if alertErr := sendFailureAlerts(client, cfg, now, err); alertErr != nil {
					log.Printf("time=%s url=%s keyword_found=%t error=%v", now.Format(time.RFC3339), cfg.WatchURL, false, alertErr)
				}
				state.FailureAlerted = true
				state.CheckedAt = now
				if saveErr := saveState(cfg.StateFile, state); saveErr != nil {
					log.Printf("time=%s url=%s keyword_found=%t error=%v", now.Format(time.RFC3339), cfg.WatchURL, false, saveErr)
				}
			}
			return
		}

		log.Printf("time=%s url=%s keyword_found=%t", now.Format(time.RFC3339), cfg.WatchURL, found)

		if found && !state.Found {
			if err := sendTelegramAlerts(client, cfg, now); err != nil {
				log.Printf("time=%s url=%s keyword_found=%t error=%v", now.Format(time.RFC3339), cfg.WatchURL, found, err)
			}
		}

		state.Found = found
		state.FailureAlerted = false
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
	loadDotEnv(".env")
	watchURL := getenv("WATCH_URL", "https://jaketboat.bankjakarta.co.id/")
	watchMethod := strings.ToUpper(getenv("WATCH_METHOD", defaultWatchMethod))
	watchBody := getenv("WATCH_BODY", defaultWatchBody)
	watchContentType := getenv("WATCH_CONTENT_TYPE", "text/plain;charset=UTF-8")
	watchHeaders := defaultWatchHeaders()
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
		WatchMethod:      watchMethod,
		WatchBody:        watchBody,
		WatchContentType: watchContentType,
		WatchHeaders:     watchHeaders,
		Keyword:          keyword,
		CheckInterval:    time.Duration(intervalSeconds) * time.Second,
		TelegramBotToken: token,
		TelegramChatIDs:  chatIDs,
		StateFile:        stateFile,
		HTTPTimeout:      time.Duration(timeoutSeconds) * time.Second,
		UserAgent:        ua,
	}, nil
}

func loadDotEnv(path string) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, val, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		val = strings.TrimSpace(val)
		if key == "" {
			continue
		}
		if _, exists := os.LookupEnv(key); exists {
			continue
		}
		_ = os.Setenv(key, strings.Trim(val, `"`))
	}
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

func defaultWatchHeaders() http.Header {
	h := http.Header{}
	// ponytail: hardcoded Next.js Server Action headers from working public request; refresh these if failure alert says request rotated.
	h.Set("Accept", "text/x-component")
	h.Set("Origin", "https://jaketboat.bankjakarta.co.id")
	h.Set("Referer", "https://jaketboat.bankjakarta.co.id/")
	h.Set("next-action", "006a9e3ed84d13a4d62ca881933c8231ac804caff2")
	h.Set("next-router-state-tree", "%5B%22%22%2C%7B%22children%22%3A%5B%22__PAGE__%22%2C%7B%7D%2Cnull%2Cnull%2C0%5D%7D%2Cnull%2Cnull%2C16%5D")
	return h
}

func checkKeyword(client *http.Client, cfg Config) (bool, error) {
	var body io.Reader
	if cfg.WatchBody != "" {
		body = strings.NewReader(cfg.WatchBody)
	}
	req, err := http.NewRequest(cfg.WatchMethod, cfg.WatchURL, body)
	if err != nil {
		return false, err
	}
	req.Header.Set("User-Agent", cfg.UserAgent)
	if cfg.WatchBody != "" {
		req.Header.Set("Content-Type", cfg.WatchContentType)
	}
	for key, values := range cfg.WatchHeaders {
		for _, value := range values {
			req.Header.Add(key, value)
		}
	}

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
	return containsKeyword(string(b), cfg.Keyword), nil
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
	message := fmt.Sprintf("Keyword ditemukan\nURL: %s\nKeyword: %s\nTime: %s", cfg.WatchURL, cfg.Keyword, now.Format(time.RFC3339))
	return sendTelegramText(client, cfg, message)
}

func sendFailureAlerts(client *http.Client, cfg Config, now time.Time, cause error) error {
	message := fmt.Sprintf("Website watcher gagal\nURL: %s\nError: %s\nTime: %s\nAction: cek apakah next-action / next-router-state-tree sudah rotate", cfg.WatchURL, cause, now.Format(time.RFC3339))
	return sendTelegramText(client, cfg, message)
}

func sendTelegramText(client *http.Client, cfg Config, message string) error {
	for _, chatID := range cfg.TelegramChatIDs {
		if err := sendTelegramMessage(client, cfg.TelegramBotToken, chatID, message); err != nil {
			return err
		}
	}
	return nil
}

func sendTelegramMessage(client *http.Client, token, chatID, message string) error {
	endpoint := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", token)
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
// ponytail: hardcoded Next.js request headers are acceptable until site rotates server-action payload.
