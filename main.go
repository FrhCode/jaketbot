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
	"sync"
	"time"
)

type runtimeConfig struct {
	mu     sync.RWMutex
	cfg    Config
	paused bool
}

func newRuntimeConfig(cfg Config) *runtimeConfig { return &runtimeConfig{cfg: cfg} }
func (r *runtimeConfig) get() Config             { r.mu.RLock(); defer r.mu.RUnlock(); return r.cfg }
func (r *runtimeConfig) set(cfg Config)          { r.mu.Lock(); r.cfg = cfg; r.mu.Unlock() }
func (r *runtimeConfig) pause()                  { r.mu.Lock(); r.paused = true; r.mu.Unlock() }
func (r *runtimeConfig) resume()                 { r.mu.Lock(); r.paused = false; r.mu.Unlock() }
func (r *runtimeConfig) isPaused() bool          { r.mu.RLock(); defer r.mu.RUnlock(); return r.paused }

type logBuffer struct {
	mu    sync.Mutex
	limit int
	lines []string
}

func newLogBuffer(limit int) *logBuffer {
	return &logBuffer{limit: limit}
}

func (b *logBuffer) add(line string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.lines = append(b.lines, line)
	if len(b.lines) > b.limit {
		b.lines = b.lines[len(b.lines)-b.limit:]
	}
}

func (b *logBuffer) tail(n int) []string {
	b.mu.Lock()
	defer b.mu.Unlock()
	if n > len(b.lines) {
		n = len(b.lines)
	}
	out := make([]string, n)
	copy(out, b.lines[len(b.lines)-n:])
	return out
}

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

	runtime := newRuntimeConfig(cfg)
	state := loadState(cfg.StateFile)
	client := &http.Client{Timeout: cfg.HTTPTimeout}
	logs := newLogBuffer(100)
	go pollTelegramCommands(client, runtime, logs)
	ticker := time.NewTicker(cfg.CheckInterval)
	defer ticker.Stop()

	check := func() {
		now := time.Now().UTC()
		cfg := runtime.get()
		if runtime.isPaused() {
			logLine(logs, "time=%s url=%s paused=true", now.Format(time.RFC3339), cfg.WatchURL)
			return
		}
		found, err := checkKeyword(client, cfg)
		if err != nil {
			logLine(logs, "time=%s url=%s keyword_found=%t error=%v", now.Format(time.RFC3339), cfg.WatchURL, false, err)
			if !state.FailureAlerted {
				if alertErr := sendFailureAlerts(client, cfg, now, err); alertErr != nil {
					logLine(logs, "time=%s url=%s keyword_found=%t error=%v", now.Format(time.RFC3339), cfg.WatchURL, false, alertErr)
				}
				state.FailureAlerted = true
				state.CheckedAt = now
				if saveErr := saveState(cfg.StateFile, state); saveErr != nil {
					logLine(logs, "time=%s url=%s keyword_found=%t error=%v", now.Format(time.RFC3339), cfg.WatchURL, false, saveErr)
				}
			}
			return
		}

		logLine(logs, "time=%s url=%s keyword_found=%t", now.Format(time.RFC3339), cfg.WatchURL, found)

		if found {
			if err := sendTelegramAlerts(client, cfg, now); err != nil {
				logLine(logs, "time=%s url=%s keyword_found=%t error=%v", now.Format(time.RFC3339), cfg.WatchURL, found, err)
			}
		}

		state.Found = found
		state.FailureAlerted = false
		state.CheckedAt = now
		if err := saveState(cfg.StateFile, state); err != nil {
			logLine(logs, "time=%s url=%s keyword_found=%t error=%v", now.Format(time.RFC3339), cfg.WatchURL, found, err)
		}
	}

	check()
	for range ticker.C {
		check()
	}
}

func logLine(logs *logBuffer, format string, args ...any) {
	line := fmt.Sprintf(format, args...)
	logs.add(line)
	log.Print(line)
}

func loadConfig() (Config, error) {
	loadDotEnv(".env")
	watchURL := getenv("WATCH_URL", "https://jaketboat.bankjakarta.co.id/")
	watchMethod := strings.ToUpper(getenv("WATCH_METHOD", defaultWatchMethod))
	watchBody := getenv("WATCH_BODY", defaultWatchBody)
	watchContentType := getenv("WATCH_CONTENT_TYPE", "text/plain;charset=UTF-8")
	watchHeaders := defaultWatchHeaders()
	applyCurlConfig(getenv("WATCH_CURL", ""), &watchURL, &watchMethod, &watchBody, &watchContentType, &watchHeaders)
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

func applyCurlConfig(raw string, watchURL *string, watchMethod *string, watchBody *string, watchContentType *string, watchHeaders *http.Header) {
	if strings.TrimSpace(raw) == "" {
		return
	}
	*watchMethod = http.MethodPost
	*watchBody = "[]"
	*watchContentType = "text/plain;charset=UTF-8"
	if parsed, ok := parseCurlURL(raw); ok {
		*watchURL = parsed
	}
	if headers, body, ok := parseCurl(raw); ok {
		for k, v := range headers {
			watchHeaders.Set(k, v)
		}
		if body != "" {
			*watchBody = body
		}
	}
}

func parseCurlURL(raw string) (string, bool) {
	idx := strings.Index(raw, "curl '")
	if idx < 0 {
		idx = strings.Index(raw, "curl \"")
		if idx < 0 {
			return "", false
		}
	}
	start := strings.Index(raw[idx:], "https://")
	if start < 0 {
		return "", false
	}
	s := raw[idx+start:]
	end := strings.IndexAny(s, "' \n")
	if end < 0 {
		end = len(s)
	}
	return s[:end], true
}

func parseCurl(raw string) (map[string]string, string, bool) {
	headers := map[string]string{}
	lines := strings.Split(raw, "\n")
	var body string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.Contains(line, "-H '") || strings.Contains(line, "-H \"") {
			parts := strings.SplitN(line, "-H ", 2)
			if len(parts) < 2 {
				continue
			}
			val := strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(parts[1]), "\\"))
			val = strings.TrimSpace(strings.Trim(val, "'\""))
			if k, v, ok := strings.Cut(val, ":"); ok {
				headers[strings.TrimSpace(k)] = strings.TrimSpace(v)
			}
		}
		if strings.Contains(line, " -b ") || strings.HasPrefix(line, "-b ") {
			if cookie := parseCurlCookie(line); cookie != "" {
				headers["Cookie"] = cookie
			}
		}
		if strings.HasPrefix(line, "--data-raw ") {
			body = strings.TrimSpace(strings.TrimPrefix(line, "--data-raw "))
			body = strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(body), "\\"))
			body = strings.Trim(body, "'\"")
		}
	}
	if len(headers) == 0 && body == "" {
		return nil, "", false
	}
	return headers, body, true
}

func parseCurlCookie(line string) string {
	idx := strings.Index(line, "-b ")
	if idx < 0 {
		return ""
	}
	v := strings.TrimSpace(line[idx+3:])
	v = strings.TrimSuffix(v, "\\")
	v = strings.TrimSpace(v)
	v = strings.TrimPrefix(v, "$")
	v = strings.Trim(v, "'\"")
	return strings.ReplaceAll(v, `\u0021`, "!")
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

type telegramUpdatesResponse struct {
	OK     bool `json:"ok"`
	Result []struct {
		UpdateID int `json:"update_id"`
		Message  *struct {
			Text string `json:"text"`
			Chat struct {
				ID int64 `json:"id"`
			} `json:"chat"`
		} `json:"message"`
	} `json:"result"`
}

func pollTelegramCommands(client *http.Client, runtime *runtimeConfig, logs *logBuffer) {
	var offset int
	for {
		cfg := runtime.get()
		updates, err := getTelegramUpdates(client, cfg.TelegramBotToken, offset)
		if err != nil {
			logLine(logs, "telegram_command_error=%v", err)
			time.Sleep(10 * time.Second)
			continue
		}
		for _, update := range updates.Result {
			offset = update.UpdateID + 1
			if update.Message == nil {
				continue
			}
			text := strings.TrimSpace(update.Message.Text)
			if !chatAllowed(update.Message.Chat.ID, cfg.TelegramChatIDs) {
				continue
			}
			switch {
			case text == "/commands":
				if err := sendTelegramMessage(client, cfg.TelegramBotToken, strconv.FormatInt(update.Message.Chat.ID, 10), commandList()); err != nil {
					logLine(logs, "telegram_command_error=%v", err)
				}
			case text == "/pause":
				runtime.pause()
				if err := sendTelegramMessage(client, cfg.TelegramBotToken, strconv.FormatInt(update.Message.Chat.ID, 10), "Paused."); err != nil {
					logLine(logs, "telegram_command_error=%v", err)
				}
			case text == "/start":
				runtime.resume()
				if err := sendTelegramMessage(client, cfg.TelegramBotToken, strconv.FormatInt(update.Message.Chat.ID, 10), "Started."); err != nil {
					logLine(logs, "telegram_command_error=%v", err)
				}
			case text == "/logs":
				lines := logs.tail(10)
				message := "No logs yet"
				if len(lines) > 0 {
					message = strings.Join(lines, "\n")
				}
				if err := sendTelegramMessage(client, cfg.TelegramBotToken, strconv.FormatInt(update.Message.Chat.ID, 10), message); err != nil {
					logLine(logs, "telegram_command_error=%v", err)
				}
			case strings.HasPrefix(text, "/curl "):
				curl := strings.TrimSpace(strings.TrimPrefix(text, "/curl "))
				next := cfg
				applyCurlConfig(curl, &next.WatchURL, &next.WatchMethod, &next.WatchBody, &next.WatchContentType, &next.WatchHeaders)
				runtime.set(next)
				logLine(logs, "watch curl updated by chat_id=%d", update.Message.Chat.ID)
				if err := sendTelegramMessage(client, cfg.TelegramBotToken, strconv.FormatInt(update.Message.Chat.ID, 10), "Curl updated for current process. Restart loses it unless WATCH_CURL is saved in .env."); err != nil {
					logLine(logs, "telegram_command_error=%v", err)
				}
			}
		}
		time.Sleep(3 * time.Second)
	}
}

func getTelegramUpdates(client *http.Client, token string, offset int) (telegramUpdatesResponse, error) {
	endpoint := fmt.Sprintf("https://api.telegram.org/bot%s/getUpdates?timeout=1&offset=%d", token, offset)
	resp, err := client.Get(endpoint)
	if err != nil {
		return telegramUpdatesResponse{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return telegramUpdatesResponse{}, fmt.Errorf("telegram getUpdates failed: %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}
	var updates telegramUpdatesResponse
	if err := json.NewDecoder(resp.Body).Decode(&updates); err != nil {
		return telegramUpdatesResponse{}, err
	}
	return updates, nil
}

func commandList() string {
	return "/commands - list command\n/pause - pause watch\n/start - resume watch\n/logs - last 10 log\n/curl ... - update request"
}

func chatAllowed(chatID int64, allowed []string) bool {
	id := strconv.FormatInt(chatID, 10)
	for _, chat := range allowed {
		if chat == id {
			return true
		}
	}
	return false
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
// ponytail: /curl command updates runtime only; persist by saving WATCH_CURL in .env.
