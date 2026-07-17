# Spec: Website Keyword Watcher

## Objective
Build small Go service that checks a website periodically, searches configured keyword case-insensitively, and sends Telegram alerts only when state changes from not found to found. User is operator monitoring `https://jaketboat.bankjakarta.co.id/` for a keyword. Success means Docker service runs, state survives restart, and no repeated alert while keyword remains present.

## Tech Stack
- Go latest stable via Docker `golang:1.24-alpine`
- Go standard library only
- Docker multi-stage build
- Docker Compose
- JSON file state at `/data/state.json`

## Commands
- Test: `go test ./...`
- Build local: `go build -o website-watcher .`
- Docker run: `docker compose up -d --build`
- Docker logs: `docker compose logs -f`

## Project Structure
- `main.go` → watcher app, config, state file, HTTP fetch, Telegram send
- `main_test.go` → runnable check for case-insensitive keyword matching
- `go.mod` → Go module
- `Dockerfile` → multi-stage image, non-root final user
- `docker-compose.yml` → `website-watcher` service and `/data` volume
- `.env.example` → required and optional env config
- `README.md` → setup, run, logs, Telegram, keyword test
- `docs/spec.md` → this spec
- `tasks/plan.md` → implementation plan
- `tasks/todo.md` → task checklist

## Code Style
```go
func containsKeyword(body, keyword string) bool {
	return strings.Contains(strings.ToLower(body), strings.ToLower(keyword))
}
```
- Small functions, no framework, no global mutable config except loaded config passed into app.
- Errors returned with context via `fmt.Errorf`.
- Logs go to stdout through standard `log` package.
- Env names match requested config exactly.

## Testing Strategy
- Framework: Go standard `testing` package.
- Tests live next to code in `main_test.go`.
- Required check: case-insensitive keyword matching.
- Verification command: `go test ./...`.
- No live Telegram or external website test in unit tests.

## Boundaries
- Always: validate required env, use HTTP timeout, use User-Agent, persist state after every successful keyword result, avoid repeated alerts while state stays true.
- Ask first: adding dependencies, adding database, changing alert semantics, changing Docker runtime base.
- Never: hardcode Telegram token/chat IDs, build UI/PWA, use external cron, send alert on non-2xx HTTP status, remove runnable check.

## Success Criteria
- `go test ./...` passes.
- `docker compose up -d --build` starts service.
- App reads config from env with documented defaults.
- App checks `https://jaketboat.bankjakarta.co.id/` periodically by default.
- App sends Telegram message to one or more chat IDs only on false-to-true transition.
- App writes and reads state from `/data/state.json` so restart does not immediately spam.

## Open Questions
- None. Assumption: current detailed prompt is approval to proceed through plan, tasks, and implementation.
