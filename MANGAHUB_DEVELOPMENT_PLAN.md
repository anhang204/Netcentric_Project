# MangaHub development plan for two students

The team has 10–11 weeks to deliver and verify F01–F06 as one working application. An owns accounts, the personal library, TCP progress synchronization and WebSocket chat. Vy owns discovery, the gRPC catalog, UDP notifications and the TUI shell. Each student owns their feature from the TUI screen to the server and tests. Use the acceptance cases in section 6 to track completion and the grading criteria in section 8 to prioritize verification. Align the last week with the announced demo date.

## 1. Assignment requirements

- Run three independent Go programs on Go 1.25 or later. `cmd/tui` uses `tview`. `cmd/backend` uses Gin for JSON REST, `net` for TCP and UDP, and Gorilla WebSocket for chat. `cmd/catalog` uses grpc-go and generated Protobuf stubs. Source code, UI labels, tests and project documentation use English.
- Keep users and reading progress in the backend SQLite database. Keep at least 30 distinct manga across at least four genres in the catalog SQLite database. Both processes use `database/sql`. Each catalog record has an ID, title, author, genres, status, total chapters and description. Seed an empty catalog once and verify persistence on restart.
- Provide keyboard navigation, visible feedback and screens for login/register, browse/details/add, library/progress, discussion, notifications and connection status/help. Network I/O runs off the TUI event loop. Incoming messages reach widgets through the event loop.
- Use local catalog metadata and reading progress as the product scope. Prepare a working demo, an English technical report and individual explanations. The project replaces the midterm and is worth 30% of the course grade.

| Feature | Required network path | Live evidence | Owner |
| --- | --- | --- | --- |
| F01 Accounts | TUI → Gin REST → backend SQLite | Register and log in, reject a wrong password, and retain the account after restart. A separate account sees its own library. | An |
| F02 Discovery | TUI → Gin REST → gRPC catalog → catalog SQLite | Search by title, select and show all detail fields, show an empty result, and report a stopped catalog gracefully. | Vy |
| F03 Library and progress | TUI → Gin REST → backend SQLite → TCP to sibling sessions | Add a manga, set one of three statuses and save a valid chapter. A second session of the same account updates visibly without refresh. The original session uses the HTTP response and a third client under another account stays unchanged. | An |
| F04 Release alerts | Admin REST trigger → Gin → gRPC `UpdateChapters` → SQLite → UDP to subscribed TUIs | Two subscribed clients see the new chapter in Notifications, an unsubscribed client stays unchanged, and a closed receiver leaves the server operational. | Vy |
| F05 Discussion | TUI ↔ authenticated Gorilla WebSocket ↔ room hub | Exchange live chat messages. A client in a separate room sees only its room, and the remaining clients can chat after a disconnect. | An |
| F06 Catalog boundary | Gin → separate gRPC process via generated stubs | Use `SearchManga`, `GetManga` and `UpdateChapters` in the connected system. Show invalid or missing IDs and a bounded HTTP error if the catalog stops. | Vy |

Source for scope, interfaces, milestones and assessment, pp. 1–8. [MangaHub Student Specification v3.0](<IT096IIU_MangaHub_Project_Student%20%281%29.pdf>)

## 2. Ownership by path

Use a single Go module rooted in `Project/`. Treat the table as the write-ownership map for implementation PRs. The other person reviews changes and implements their own code against the agreed interfaces.

| Path | Maintainer | Work |
| --- | --- | --- |
| `cmd/backend/`, `internal/backend/bootstrap/` | An | Start and wire Gin, TCP and UDP, register routes, configure dependencies and shut down listeners. Vy hands An the registration functions for discovery, releases and alerts. |
| `internal/backend/{accounts,library,progress,chat,store}/` | An | Password hashing, JWT validation, users and library schema, progress transactions, TCP session hub and WebSocket room hub. |
| `internal/tui/{accounts,library,chat}/` | An | Login/register, library and progress, chat screens, and the TCP and WS clients consumed by those screens. |
| `cmd/catalog/`, `internal/catalog/`, `proto/`, `gen/catalogpb/` | Vy | Catalog schema and seed, gRPC server, `.proto`, generated stubs and generation command. |
| `internal/backend/{catalogclient,discovery,releases,alerts}/` | Vy | Gin-to-gRPC client, discovery routes, protected release flow, UDP subscriptions and broadcasts. |
| `cmd/tui/`, `internal/tui/{shell,discovery,notifications}/` | Vy | TUI startup, navigation/help/connection status, discovery and details, notification view and UDP client. Vy wires An's screen constructors into the shell. |
| `internal/contract/`, `docs/network-contract.md`, `go.mod`, `go.sum`, `.gitignore`, `.env.example`, `README.md` | An | Shared JSON types, configuration names, toolchain and dependencies. Vy reviews contract and tooling changes and requests needed dependencies. |
| `docs/report/backend-notes.md` | An | Draft the architecture and progress flow, document HTTP/TCP/WS behavior, and record evidence for An's tests. |
| `tests/e2e/`, `scripts/`, other files in `docs/report/`, `REPORT.pdf` | Vy | Own integrated verification and the demo runner. Document gRPC and UDP, assemble the report, and request An's factual review. Both own the live demo. |

Unit and package integration tests live beside their feature code and follow the same ownership. Use Lab 05 for examples of Gin routes, SQLite and restart tests. Use Lab 06 for examples of stub generation, unary RPCs and separate-process tests. [Lab 05 reference](../Lab/Lab05/README.md) · [Lab 06 reference](../Lab/Lab06/README.md)

## 3. Week 1 handoffs and contracts

Review the routes, representative messages, error codes, env names and interfaces together. An merges the module path and shared HTTP/JSON contract. Vy merges the initial `.proto`, generated stubs and catalog-client interface implementation before An wires real gRPC into the backend. Anyone waiting on an implementation can write package tests against a fake using the approved contract. The integrated TUI uses running services.

| Producer | Consumer | Handoff to agree and test |
| --- | --- | --- |
| Accounts (An) | Library, TCP and chat (An), UDP and release (Vy) | JWT verifier returns trusted `UserID` and `SessionID`. Each successful login creates a fresh session ID. Supply test tokens for two logins of one user and one login of another user. |
| Catalog client (Vy) | Library (An), discovery and release (Vy) | `GetManga(ctx, id)` yields catalog details and the chapter limit. An defines the narrow interface the library consumes. Vy exposes functions for search and chapter update and documents timeout/error behavior. |
| Library and screens (An) | TUI shell and discovery (Vy) | Discovery calls the library add action with `manga_id`. An exposes constructors for account, library and chat screens. Vy documents the shell callbacks for login state, navigation and safe UI updates. |
| Discovery, release and alerts (Vy) | Backend startup (An) | Vy provides route registration and UDP start/stop entry points. An passes configuration, the token verifier and dependencies at startup. Catalog runs as its own process. |

### REST and stored data

| Method and path | Proposed payload or result |
| --- | --- |
| `POST /api/auth/register` | `{username,password}` → `201`. Duplicate username → `409`. Store a password hash. |
| `POST /api/auth/login` | `{username,password}` → `200` with `{token}`. Bad credentials → `401`. |
| `GET /api/manga?q=...` | `200` with `{items:[...]}`. Empty `q` lists the catalog. A search with zero matches returns an empty array. |
| `GET /api/manga/:id` | `200` with `{id,title,author,genres,status,total_chapters,description}`. |
| `GET /api/library` | `200` with `{items:[...]}` for the authenticated account, with `manga_id`, `status`, `current_chapter`, `updated_at` and `version` per entry. |
| `POST /api/library` | `{manga_id}` → `201` with new entry at chapter `0` and status `plan-to-read`. Duplicate entry → `409`. |
| `PUT /api/library/:mangaId/progress` | `{current_chapter?,status?}` with at least one field → `200` with the saved entry. Status is `plan-to-read`, `reading` or `completed`. Chapter accepts integers from `0` through the catalog total. An entry missing from the signed-in user's library returns `404`. |
| `POST /api/admin/manga/:id/release` | `{total_chapters}` with a higher count → `200` with updated manga and a UDP alert. Require a JWT plus an `X-Admin-Demo-Key` header matching the configured demo key. |
| `GET /api/health` and `GET /ws` | Backend health returns JSON. Chat upgrades an authenticated request. |

| Table | SQLite owner | Required fields and proposed constraints |
| --- | --- | --- |
| `users` | Gin backend | `id` primary key, unique `username`, `password_hash`, `created_at`. Hash validated passwords before writing. |
| `user_library` | Gin backend | `user_id`, `manga_id`, `status`, `current_chapter`, `updated_at`, `version`. Unique pair `(user_id, manga_id)`, with `user_id` referencing `users.id`. Constrain status values and require a nonnegative chapter. Validate the upper bound through gRPC. |
| `manga` | gRPC catalog | `id` primary key, `title`, `author`, `genres`, `status`, `total_chapters`, `description`. Seed stable IDs with all fields filled and require a nonnegative chapter count. Apply each release as an atomic increase. |

Protected HTTP and WebSocket requests use `Authorization: Bearer <token>`. Verify the JWT signature and expiration and derive account and session IDs from its verified claims. Each successful progress write increments `version` atomically. A failed write leaves the saved state intact and produces zero events. The backend obtains catalog data through gRPC. `GET /api/library` remains usable when the catalog is offline. The TUI displays known manga titles and falls back to IDs if a lookup is unavailable.

Use a consistent JSON error envelope such as `{"error":{"code":"catalog_unavailable","message":"The manga catalog is unavailable."}}`. Use `400` for invalid input, `401` for missing/invalid credentials, `403` for denied permissions, `404` for a missing resource, `409` for conflicts and `503` for catalog unavailability. The gRPC client maps `InvalidArgument` to `400`, `NotFound` to `404`, and `Unavailable` or `DeadlineExceeded` to `503`. Start with a configurable two-second deadline on every Gin-to-catalog call.

The `.proto` defines three unary calls. `SearchManga` takes a title query and returns a repeated list. `GetManga` takes a manga ID and returns a record. `UpdateChapters` takes the manga ID and proposed total and returns the committed record. Define field numbers and a common manga message in week 1. An invalid or empty ID returns `InvalidArgument`. A valid unknown ID returns `NotFound`. An invalid chapter update leaves the catalog unchanged.

### TCP, UDP and WebSocket

- **TCP** uses one JSON object per newline. The first frame is `{"type":"auth","token":"..."}`. After a successful HTTP commit, send `{"type":"progress_updated","manga_id":"one-piece","chapter":25,"status":"reading","version":2}` to connections with the same verified user ID and a different session ID. Serialize writes per connection, bound writes so a stalled reader cannot block progress saves, and remove disconnected sessions. Apply the same per-entry version check to saved HTTP responses, TCP events and library snapshots so a delayed response cannot overwrite a newer update. Fetch the library once after reconnecting to recover missed state.
- **UDP** uses one JSON object per datagram. The TUI sends `subscribe` or `unsubscribe` with its JWT from the socket that listens for alerts. The backend records the datagram source address, refreshes subscriptions idempotently, and expires stale entries. Use a short `subscribed` acknowledgment and a TUI timeout so the screen can distinguish registered and pending. Renew the subscription before it expires while the user is subscribed. A release first commits in the catalog, then the backend sends `{"type":"new_chapter","manga_id":"one-piece","chapter":26}` to active subscribers. Delivery is best-effort.
- **WebSocket** uses `join` and `chat` JSON messages carrying a `room` field. The server sends confirmation of the joined room, validates each send against that membership, derives the sender from the JWT, and broadcasts within the room. Serialize each client's writes, bound its send queue and remove it on disconnect. The TUI receives messages asynchronously while the input stays usable.

Bind the TUI's network connections to its login session. On logout, unsubscribe from UDP, close its sockets and clear its screen state before showing the login screen. Verify that a subsequent login receives only the new session's events.

## 4. Git, local builds and integration

1. An creates a Git repository for the `Project/` directory and shares it with Vy. Each person works in their own clone, or a separate worktree when sharing a machine. Start from `main` and use short-lived branches such as `feat/an-library` and `feat/vy-catalog`. Keep `main` buildable.
2. For a shared contract or config change, merge a small PR first and sync the dependent branches with `main`. If a feature adds an import, An updates `go.mod` and `go.sum` in a separate commit on that feature PR and runs `go mod tidy` with the imports present. Merge dependency changes one PR at a time. Vy is the sole writer of `.proto` and generated stubs and commits both together. An interface change documents the impact on its consumer and includes an updated example or test.
3. Before review, sync with `main`, inspect changed paths with `git diff --name-only origin/main...HEAD` and run the checks below. The other student reviews the PR. If branches intersect, the file owner resolves the conflict, reruns the relevant tests and publishes the resolution for both students to review.
4. The owner of each entry point performs the final wiring after the provider PR lands. An registers Vy's Gin routes and UDP listener. Vy mounts An's screens. Run the live acceptance test for that slice before moving on. Keep the tracked `.env.example` as a reference and configure actual addresses and database paths per local instance.

| Instance | HTTP and WS | TCP | UDP | gRPC | SQLite paths |
| --- | --- | --- | --- | --- | --- |
| Standard demo | `127.0.0.1:8080` | `127.0.0.1:9090` | `127.0.0.1:9091` | `127.0.0.1:9092` | `data/demo/users.db`, `data/demo/catalog.db` |
| Second stack on the same machine | `127.0.0.1:18080` | `127.0.0.1:19090` | `127.0.0.1:19091` | `127.0.0.1:19092` | `data/dev-vy/users.db`, `data/dev-vy/catalog.db` |
| Automated tests | Temporary listeners | Temporary listeners | Temporary listeners | Temporary listeners | Per-test temp directory |

The sample config documents `API_ADDR`, `TCP_ADDR`, `UDP_ADDR`, `CATALOG_ADDR`, `CATALOG_RPC_TIMEOUT`, `USERS_DB_PATH`, `CATALOG_DB_PATH`, `JWT_SECRET` and `ADMIN_DEMO_KEY`. The README shows how to apply those settings to each process terminal. Keep runtime settings, databases and SQLite sidecars, local experiment logs and build outputs in ignored paths. Place demo binaries in the `bin/` directory of the local checkout. For independent builds use a separate stack per person. For TCP, UDP and room-isolation verification, point all participating TUIs at **one** integration stack and give each TUI its own UDP socket.

At kickoff, check the Go, GCC and `protoc` toolchains on both machines. The proposed SQLite driver follows the Lab 05 `go-sqlite3` setup and requires GCC with CGO enabled. Vy records the plugin versions and generation command for `protoc-gen-go` and `protoc-gen-go-grpc` and checks that regeneration leaves the committed stubs unchanged. Once the three command packages exist, run from the project root:

```powershell
go build ./cmd/backend ./cmd/catalog ./cmd/tui
go test -count=1 ./...
go vet ./...
go test -race -count=1 ./...
```

Run the race test on a supported environment and record any toolchain limit with the verification results. Run `gofmt` on changed handwritten Go files before opening a PR. The clean-run order is catalog, backend, then TUI clients, using `go run ./cmd/catalog`, `go run ./cmd/backend` and `go run ./cmd/tui` in separate terminals after applying the environment settings for that stack.

## 5. Weekly milestones

### First week work and merge sequence

1. **Day 1.** An creates the shared repo, commits the module path and records the runtime configuration names. Vy reviews them and sketches the catalog RPCs and TUI screen boundaries in parallel.
2. **Day 2.** An and Vy agree on JSON examples, identity fields, catalog errors and event delivery. An lands the shared contract. Vy submits the `.proto`, generated stubs and generation command. Each person can then write tests against the interface they will consume.
3. **Days 3–4.** An builds the Gin health entry point and auth/storage skeleton. Vy builds the catalog startup and keyboard-driven TUI shell. Merge the provider packages before importing them from the owner’s entry point. An serializes changes to the Go module locks as new imports land.
4. **Day 5.** Both build the three entry points from `main`. An checks cold startup from a clean clone and Vy checks startup with the second port/database set. Record the starting commit and split the next week into feature PRs with an owner, paths, dependency and acceptance test for each PR.

### Weeks 1–11

| Week | An | Vy | Merge order and evidence |
| --- | --- | --- | --- |
| 1 | Initialize the module and repo, configure Gin health and draft shared auth/JSON contracts. | Build the `tview` shell, define the `.proto`, generate stubs and agree on catalog data and UI handoffs. | Merge module/contract, then catalog stubs and separate skeleton PRs. Prove all three entry points build and start. |
| 2 | Add SQLite user schema, register/login, JWT verification and login/register screens. Start library schema and tests. | Build catalog schema and seed, `SearchManga`/`GetManga`, a minimal Gin search handler and browser/detail screens. | Merge auth, then mount An's screens. Merge Vy's search provider, then An wires the route. Verify sign-in and a real Gin-to-catalog search before library integration. |
| 3 | Deliver library add/list/status/chapter updates and the library screen. Wire in Vy's catalog client. | Finish Gin discovery, the TUI add handoff and bounded gRPC error handling. Implement `UpdateChapters`. | Land the catalog adapter before An's wiring. Accept login → search → detail → add → change chapter on `main`. |
| 4 | Deliver and test TCP auth, newline framing, same-account routing and UI updates. | Implement UDP subscriptions, notifications and the protected release handler. | Merge TCP first and prove two same-account sessions plus one other account. Wire and smoke-test Vy's release endpoint. |
| 5 | Build the authenticated WebSocket hub and chat screen, with server-side room enforcement. | Wire UDP end to end and test release persistence, two subscribers and one unsubscribed client. | Vy's UDP provider lands before An enables it in backend startup. Demo a release with three TUIs. |
| 6 | Finish chat disconnect handling and multi-client tests. | Mount An's chat screen, finish help/connection status and assemble an initial demo runner. | Merge chat before shell mounting. Demonstrate F01–F06 through the connected application by the end of the week. |
| 7–8 | Test invalid/unauthorized operations, concurrent progress and slow/disconnected TCP or WS clients. Fix An's failures. | Test stopped catalog, deadline and recovery, UDP unavailable receiver, clean database initialization and restart. Fix Vy's failures. | Run the complete protocol test matrix from a clean clone. Cross-review one failure case in each other's code. |
| 9–10 | Verify restart and isolation on the demo environment. Write architecture and HTTP/TCP/WS rationale. Own dependency and README clean-up. | Record measured outcomes, assemble diagrams and `REPORT.pdf`. Own the end-to-end rerun and verify a draft package from an extracted archive. | Jointly select one real defect and three meaningful AI interactions for the report. Rerun the required checks and reserve the rehearsal before starting any optional Redis work. |
| 11 | Assemble the final ZIP and lead the account/library, TCP and chat portion of rehearsal. | Lead catalog/UDP and the failure portion, and repeat the demo from fresh state. | Rehearse with both students, verify the extracted ZIP from scratch, and submit by the announced deadline. If the window is 10 weeks, combine week 11 with the end of week 10. |

## 6. Verification and acceptance

For each case, record the commit, environment, setup or command, expected result, observed result and pass/fail status. Leave observations pending until measured. Use synthetic accounts and temporary databases for repeatable tests. Verify a fresh start after extracting the candidate ZIP on each student's machine.

### System and data

| Case | Owner | Pass condition and evidence |
| --- | --- | --- |
| SYS-01 Toolchain and processes | An + Vy | Build all three entry points with Go 1.25 or later. Start the catalog, backend and TUI as separate programs. Record ports, processes and the commit. Regeneration produces the committed Go stubs. |
| SYS-02 Catalog seed and ownership | Vy | Start with an empty catalog database. Query at least 30 distinct IDs and at least four genres. Inspect all F02 fields and confirm that restart creates zero duplicate records. Show that Gin gets its catalog responses from the separate process. |
| SYS-03 TUI usability | Vy with An | Navigate login/register, browser/details/add, library/progress, discussion, notifications and connection status/help using the keyboard. Show feedback for loading, success and failure. Keep navigating or typing while TCP, UDP and chat events arrive. Inspect that background receivers dispatch through the TUI event loop. |
| SYS-04 Backend persistence | An | Register, add a manga, change status and save a chapter. Restart the backend with the same database path, then log in and retrieve the exact saved status, chapter and version. |
| SYS-05 Catalog persistence | Vy | Release a chapter, restart the catalog with the same database path, then retrieve the updated total through Gin and the TUI detail screen. |

### Feature cases by owner

| Case | Protocol and owner | Pass condition and evidence |
| --- | --- | --- |
| F01-01 Account flow | HTTP, An | Register and log in through the TUI. Verify `201`/`200`, a usable JWT and a stored password hash. Record statuses and redacted results. |
| F01-02 Validation | HTTP, An | Blank or malformed form → `400`. Wrong password or missing, expired or invalid token on a protected operation → `401`. Duplicate username → `409`. The TUI shows readable errors. |
| F01-03 Account isolation | HTTP, An | A different account sees only its own entries. Attempting to update a manga that exists only in the first account's library returns a missing-entry error and leaves that record unchanged. Test a forged `user_id` field and verify the token identity still governs the result. |
| F02-01 Search and detail | HTTP → gRPC, Vy | Search by title, select a result and verify title, author, genres, status, total chapters and description on screen. Trace `SearchManga` and `GetManga` at both processes. |
| F02-02 Empty and missing | HTTP → gRPC, Vy | A query with zero matches returns `200` and an empty list. An unknown ID returns `404`. Show both states without freezing the TUI. |
| F03-01 Library and validation | HTTP, An | Add a manga and list it. Exercise all three statuses. Save chapters `0` and `total_chapters`. Negative, fractional or excessive chapters and invalid statuses return `400`. Duplicate add returns `409`. The previously saved record remains intact after a rejected update. |
| F03-02 Live sync and privacy | TCP, An | TUI 1 and TUI 2 log in separately as one account. TUI 3 uses another account. Save a chapter in TUI 1 and capture TUI 2 updating from TCP. Check that TUI 2 makes zero polling or manual refresh requests during delivery. Verify zero private events in TUI 3 and zero echo to the originating TCP session. TUI 1 uses the saved HTTP response. |
| F03-03 Framing and disconnect | TCP, An | Use valid newline framing across split and coalesced reads. Reject a missing/invalid first authentication frame within a bounded wait. Disconnect the sibling, save again and reconnect. Verify cleanup and that the reconnected TUI loads the latest chapter. |
| F03-04 Concurrent saves | HTTP + TCP, An | Send simultaneous valid writes from the two same-account sessions. Verify an atomic increment for each committed write and agreement between the final SQLite record and both TUI views. A failed save emits zero events. |
| F04-01 Subscribe and release | UDP + gRPC, Vy | Subscribe from two logged-in TUIs and leave a third unsubscribed. Trigger a protected chapter release. Show the new chapter inside both Notifications views, no alert in TUI 3, and the new total in catalog SQLite. |
| F04-02 Authorization and cleanup | UDP + HTTP, Vy | Invalid credentials leave the sender unsubscribed. Start with three subscribers, unsubscribe one and close a second without unsubscribing. Release again and verify a notification at the third client, no datagram at the unsubscribed client, and a healthy backend. A valid JWT with a wrong admin key returns `403`. |
| F04-03 Failed release | gRPC + UDP, Vy | Attempt a decreasing/invalid chapter total or a release while the catalog is stopped. Verify the error and zero notification datagrams. Check the old committed total after recovering from the outage. |
| F04-04 UDP timeout and loss | UDP, Vy | Drop a subscription acknowledgment in a controlled test. The TUI remains usable, shows the subscription timeout, and can retry. Record that UDP delivery is best-effort in the report. |
| F05-01 Live room chat | WebSocket, An | Verify an authenticated upgrade. Alice and Bob join `general` and exchange messages with the TUI still interactive. Charlie joins another room and sees only that room's events. |
| F05-02 Room and session lifecycle | WebSocket, An | Reject unauthenticated upgrades and sends to a room the session has not joined. Disconnect Bob, then have another client join `general` and show that remaining and new clients can still receive messages. Verify Bob was removed from server membership. |
| F06-01 Typed RPCs and errors | gRPC, Vy | Exercise unary `SearchManga`, `GetManga` and `UpdateChapters` over the generated client. An invalid ID/update returns `InvalidArgument`, an unknown ID returns `NotFound`, and each is mapped to the agreed HTTP response. |
| F06-02 Bounded outage and recovery | gRPC + HTTP, Vy | Stop the catalog and search from the TUI. Measure the `503` and compare it with the configured deadline. Repeat against an unresponsive test service to exercise deadline expiration. Restart the catalog and repeat the original search successfully. |

A feature PR is ready after its owner verifies the case IDs it touches and the other student reviews the code. A release candidate is ready after SYS-01 through SYS-05 and F01 through F06 pass, both students reproduce the four verification commands in section 4 on supported tooling, and the live behavior matches the report. Vy records results in one evidence table. An supplies observations for the cases he owns. Record any environmental limit that affects a verification command.

## 7. Demo, report and submission

### Live run, about 7 minutes

Prepare separate terminals for catalog and backend plus three TUI sessions. Prebuild the binaries and prepare a repeatable demo state using synthetic accounts and the documented seed. Rehearse starting and stopping each process independently so either student can handle a reordered check.

| Elapsed time | Lead | Action and observable result |
| --- | --- | --- |
| 0:00–0:35 | Vy | Start catalog, backend and TUI separately. Point out the two databases and configured listener addresses. |
| 0:35–1:55 | An, then Vy | Show an account login and a wrong-password error. From the TUI, Vy searches by title and opens details. An adds the manga, sets `reading` and opens the same library in a second login. Keep a separate registration and empty-search example ready for the extended checks. |
| 1:55–2:55 | An | Save a valid chapter in the first TUI and show the second changing without refresh. The third TUI, on another account, stays unchanged. Submit a chapter above the total and show the validation error. |
| 2:55–4:10 | Vy | Subscribe the first two sessions and leave the third unsubscribed. Use the protected demo trigger to release a chapter and show both notifications. Close the second receiver without unsubscribing and trigger another release to show continued delivery to the first. |
| 4:10–5:15 | An | Reopen the second session as Bob. Alice and Bob chat in `general` while Charlie sees a separate room. Disconnect Bob, then join Charlie to `general` to prove the hub still delivers messages. |
| 5:15–6:10 | Vy | Trace the Gin-to-catalog RPC, stop the catalog, show the bounded TUI error and restart the catalog. Repeat search or details and confirm the published chapter remains. |
| 6:10–7:00 | An | Restart the backend, log in again and show the saved library status and chapter. Point to the recorded normal, failure and persistence tests. |

**Technical Q&A, about 3 minutes.** An explains the progress transaction, TCP framing and WebSocket cleanup. Vy explains gRPC process boundaries and deadlines, and why UDP delivery is best-effort. Each student practices an AI-off explanation of one path owned by the other.

**Adaptation or debug, about 2–5 minutes.** Both students participate. Practice one change on a disposable branch, such as enforcing the upper chapter bound or adjusting the gRPC deadline. One student reproduces the symptom and locates the code, the other proposes the smallest change and runs a targeted check. Swap roles on the next practice. Both attend the demo or secure advance approval for any exception as described in the specification.

### Technical report

Deliver one English `REPORT.pdf` with approximately 4–5 pages of main content, excluding cover, references and brief appendices. The sequence and verification tables should show measured results from the build being demonstrated. Use a few relevant screenshots at most.

| Section from the specification | Owner | Content and indicative space |
| --- | --- | --- |
| 1. System Architecture | An drafts, Vy checks | About 1 page. Label `tview`, Gin, TCP, UDP, WebSocket, catalog gRPC and both SQLite owners on one diagram. Label Redis if included. Use the actual processes and ports. |
| 2. Why These Technologies? | Both | About 1 page. An writes HTTP, TCP and WebSocket. Vy writes UDP, gRPC and Redis if selected. For each, cover the feature, actual data flow, protocol choice and a concrete failure mode or limitation. Explain TCP as a byte stream and how the app handles disconnects. |
| 3. Implementation & Verification | An draws progress, Vy draws release and assembles evidence | About 1–2 pages. Draw the HTTP commit and same-account TCP fan-out, then the gRPC release commit and UDP fan-out. Include a compact table with setup, expected and observed outcomes for normal, negative, outage and multi-client cases. Link to reproducing commands or test IDs. |
| 4. Challenges, Reflection & Contributions | Both write, Vy assembles | About 1 page. Document one real defect with symptom, root cause, fix and measured retest. Include limitations, what each student built and verified, and three meaningful AI uses. At least one AI entry explains a specific suggestion that was tested, corrected or rejected and the verification behind that decision. |

Record AI uses as work happens. For each, capture the task, the suggestion, the team's decision and the specific test or review used to verify it. Both students should be able to explain their design decisions in person.

### Submission checklist

- [ ] An assembles `GroupXX_MangaHub.zip` using the assigned group number. Include Go source for all three commands and packages, `go.mod`, `go.sum`, `.proto`, generated stubs, SQLite schemas, seed data and tests.
- [ ] Vy supplies the final English `REPORT.pdf`. Both check the reported results, process boundaries, port numbers and contributions against the release commit.
- [ ] An supplies the short `README.md` with prerequisites, launch commands for all three processes, ports, required services, synthetic demo accounts and known limitations.
- [ ] An includes a secret-free `.env.example`. Review archive contents for personal data and actual secrets. Use seed fixtures and create SQLite files when launching the extracted project.
- [ ] Vy tests the ZIP after extraction to a clean directory, starting first with empty data paths and then restarting with the same paths. Both rehearse the complete demo from the release candidate.
- [ ] Submit the single ZIP through Blackboard by 23:59 on the announced demo day unless the instructor gives another deadline. An handles submission and Vy checks the uploaded filename and receipt.

## 8. Grading criteria and optional Redis

The specification evaluates the connected implementation, the English technical report and both students' understanding holistically. The criteria below describe evidence the team can prepare for each performance level.

| Performance level | Evidence or gap to identify |
| --- | --- |
| Below B− | The project lacks a reproducible connected application, substantial required work, or an adequate live demonstration. Use the clean-build and rehearsal checks to detect these gaps early. |
| B− | Go TUI talks to Go/Gin REST and at least some account, search and library workflows work. |
| B | The connected TUI, persistence and several protocol workflows work, with at least one major reliability or integration gap remaining. |
| B+ | F01–F06 work through the real TUI across all five protocols, with the basic user workflows and persistence demonstrated. |
| A− | Add repeatable client isolation, validation, disconnect cleanup and controlled catalog failure/recovery. Explain each choice in the report. |
| A | Bring systematic positive, negative and concurrency evidence, one investigated failure, and convincing technical explanations from both students. |
| A+ | Demonstrate exceptional depth in fault isolation, concurrency, a live adaptation or verified Redis behavior. Choose the priority once the required feature matrix passes. |

For advanced fault and concurrency evidence, An can verify simultaneous progress saves while an authenticated sibling stops reading and a different account stays connected. Check commit count, final version, recovery when the sibling reconnects and continued account isolation. Vy can record the observed elapsed time for an unresponsive catalog, continued access to saved library data, recovery after restart and absence of alerts for a failed release. Rehearse one of these as a live diagnosis and record the exact results in the report.

### F07 Optional Redis workstream

Start after the required workflows and a complete rehearsal pass. Vy owns the cache code and tests under `internal/backend/discovery/`, plus the release hook under `internal/backend/releases/`. An owns any `go.mod`, `go.sum` and sample config changes and reviews the cache PR. Use `go-redis/v9` and a configurable TTL with a 30-second default.

1. For `GET /api/manga/:id`, look up the manga detail key and return `X-Cache: HIT` on a valid Redis result. On a miss, fetch through `GetManga`, store a successful result with a TTL and return `X-Cache: MISS`. Keep catalog SQLite authoritative and use bounded Redis operations.
2. After `UpdateChapters` commits, invalidate that manga's key before acknowledging the release. Coordinate detail cache fills and invalidation for that manga so a concurrent lookup cannot repopulate the old value. Keep the UDP announcement tied to the successful catalog commit.
3. If Redis is unreachable, get details directly from gRPC and treat the lookup as a miss. If post-release invalidation fails, bypass the affected entry until it is deleted or its TTL elapses. Use a per-backend-start key namespace if implementing the bypass in memory so a restart cannot revive the older entry. Record the chosen behavior in the report and README.
4. Verify `MISS → HIT → release → MISS` and confirm the higher chapter in the last payload. Separately verify TTL expiry, a concurrent lookup during release and a stopped Redis process. Capture `X-Cache`, the observed chapter count and what the TUI shows. Add Redis to the diagram, protocol rationale and demo only if this workstream passes.

## 9. Specification coverage

| Source in the student specification | Where to verify it in this plan |
| --- | --- |
| Sections 1–2, pp. 1–2 | Fixed stack, three programs, five protocol jobs and TUI integration in plan sections 1 and 3. System build and UI checks in SYS-01 and SYS-03. |
| Section 3, pp. 2–3 | F01–F06 acceptance cases and live evidence in plan sections 6–7. Optional F07 build and verification in plan section 8. |
| Sections 4–5, pp. 3–5 | Screen and keyboard acceptance in SYS-03. Routes, framing, message types, deadlines and stored fields in plan section 3. Dataset and persistence in SYS-02, SYS-04 and SYS-05. |
| Section 6, pp. 5–6 | Performance levels, evidence priorities and an advanced test plan in plan section 8. Individual ownership in plan sections 2 and 7. |
| Section 7, pp. 6–7 | Required English technical report, its four sections, source evidence and PDF assembly in plan section 7. |
| Section 8, p. 7 | Required normal and negative tests, concurrent clients, restart and unauthorized checks in plan section 6. Timed demo, Q&A and live adaptation in plan section 7. |
| Section 9, p. 8 | Weekly gates in plan section 5. Archive, deadline, AI record and release checklist in plan section 7. |
