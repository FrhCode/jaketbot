# Tasks: Website Keyword Watcher

## Task 1: Create module, matching function, and test
**Description:** Create Go module and runnable test for case-insensitive keyword matching.

**Acceptance criteria:**
- [ ] `go.mod` exists.
- [ ] `containsKeyword` matches keyword regardless of case.
- [ ] `containsKeyword` returns false when keyword absent.

**Verification:**
- [ ] `go test ./...`

**Dependencies:** None

**Files likely touched:**
- `go.mod`
- `main.go`
- `main_test.go`

**Estimated scope:** Small: 3 files

## Task 2: Add config loading and validation
**Description:** Load env vars, apply defaults, validate required vars and numeric values.

**Acceptance criteria:**
- [ ] Required `KEYWORD`, `TELEGRAM_BOT_TOKEN`, `TELEGRAM_CHAT_IDS` enforced.
- [ ] Defaults match spec.
- [ ] Chat IDs parse comma-separated values.

**Verification:**
- [ ] `go test ./...`
- [ ] `go build -o website-watcher .`

**Dependencies:** Task 1

**Files likely touched:**
- `main.go`

**Estimated scope:** Small: 1 file

## Task 3: Add state, fetch, check, and logging
**Description:** Implement state JSON load/save, HTTP GET with timeout and User-Agent, non-2xx skip, keyword found logging.

**Acceptance criteria:**
- [ ] Missing state file means prior found false.
- [ ] Successful checks save found true/false.
- [ ] Non-2xx logs error and sends no alert.

**Verification:**
- [ ] `go test ./...`
- [ ] `go build -o website-watcher .`

**Dependencies:** Task 2

**Files likely touched:**
- `main.go`

**Estimated scope:** Medium: 1 file

## Task 4: Add Telegram false-to-true alerts
**Description:** Send Telegram message to every chat ID only when found changes from false to true.

**Acceptance criteria:**
- [ ] Message format matches spec.
- [ ] Multiple chat IDs receive sends.
- [ ] True-to-true sends no message.

**Verification:**
- [ ] `go test ./...`
- [ ] `go build -o website-watcher .`

**Dependencies:** Task 3

**Files likely touched:**
- `main.go`

**Estimated scope:** Medium: 1 file

## Task 5: Add Docker runtime files
**Description:** Add multi-stage Dockerfile, Compose service, and `.env.example`.

**Acceptance criteria:**
- [ ] Final image small and non-root.
- [ ] Compose service named `website-watcher`.
- [ ] Volume `watcher_data:/data` mounted.

**Verification:**
- [ ] `docker compose config`

**Dependencies:** Task 4

**Files likely touched:**
- `Dockerfile`
- `docker-compose.yml`
- `.env.example`

**Estimated scope:** Small: 3 files

## Task 6: Add README
**Description:** Document Telegram bot setup, chat ID lookup, env setup, run, logs, and keyword test.

**Acceptance criteria:**
- [ ] README covers all requested sections.
- [ ] Commands are copy-pasteable.

**Verification:**
- [ ] Read README against spec checklist.

**Dependencies:** Task 5

**Files likely touched:**
- `README.md`

**Estimated scope:** Small: 1 file
