# Website Keyword Watcher

Small Go app that checks website keyword and sends Telegram alert when keyword appears.

## Telegram bot
1. Open `@BotFather` in Telegram.
2. Send `/newbot`.
3. Follow prompt, get bot token.

## Get chat_id
1. Send `/start` to bot.
2. Open:
   `https://api.telegram.org/bot<TOKEN>/getUpdates`
3. Read `chat.id` from response.

## `.env`
Copy `.env.example` to `.env` and fill. The app reads `.env` automatically when run from project folder.
- `KEYWORD` required
- `TELEGRAM_BOT_TOKEN` required
- `TELEGRAM_CHAT_IDS` required, comma-separated
- Optional defaults already set for other vars

Example:
```env
WATCH_URL=https://jaketboat.bankjakarta.co.id/
KEYWORD=jadwal
CHECK_INTERVAL_SECONDS=300
TELEGRAM_BOT_TOKEN=123:abc
TELEGRAM_CHAT_IDS=123456,789012
STATE_FILE=/data/state.json
HTTP_TIMEOUT_SECONDS=20
USER_AGENT=Mozilla/5.0 WebsiteKeywordWatcher/1.0
```

## Run
```bash
docker compose up -d --build
```

## Logs
```bash
docker compose logs -f
```

## Test keyword
1. Set `KEYWORD` to text that exists on target page.
2. Start app.
3. Watch logs for `keyword_found=true`.
4. Telegram alert send only on false-to-true change.

## Local test
```bash
go test ./...
```
