# Implementation Plan: Website Keyword Watcher

## Overview
Build one small Go binary using standard library. It loads env config, loops on interval, fetches target URL, matches keyword case-insensitively, stores last found state in JSON, and sends Telegram only on false-to-true transition. Docker Compose runs it with `/data` volume.

## Architecture Decisions
- Standard library only: `net/http`, `encoding/json`, `os`, `time`, `strings` cover MVP.
- Single `main.go`: app is small; extra packages add ceremony.
- State schema: `{ "found": bool, "checked_at": "RFC3339" }` enough to suppress restart spam.
- Telegram send uses HTTP POST form to `/sendMessage`; no Telegram SDK needed.
- Ticker loop runs first check immediately, then interval.

## Task List

### Phase 1: Go foundation
- [ ] Task 1: Create module, matching function, and test.
- [ ] Task 2: Add config loading and validation.

### Checkpoint: Foundation
- [ ] `go test ./...` passes.

### Phase 2: Watcher behavior
- [ ] Task 3: Add state load/save, website fetch, keyword check, loop logging.
- [ ] Task 4: Add Telegram send and false-to-true alert behavior.

### Checkpoint: Core Features
- [ ] `go test ./...` passes.
- [ ] `go build -o website-watcher .` succeeds.

### Phase 3: Docker and docs
- [ ] Task 5: Add Dockerfile, Compose, env example.
- [ ] Task 6: Add README with Telegram setup, run, logs, and keyword test.

### Checkpoint: Complete
- [ ] `go test ./...` passes.
- [ ] `docker compose config` succeeds.
- [ ] Acceptance criteria covered.

## Risks and Mitigations
| Risk | Impact | Mitigation |
|------|--------|------------|
| Website blocks default Go user agent | Medium | Configurable `USER_AGENT`, default browser-like string |
| Telegram credentials invalid | Medium | Log send errors, never hardcode secrets |
| State file missing on first run | Low | Treat as `found=false` |
| Non-2xx response contains keyword in error page | Medium | Skip keyword alert on non-2xx status |

## Open Questions
- None.
