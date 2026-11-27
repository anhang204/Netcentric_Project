# An's backend and TUI notes

## Technical report handoff

The student specification, section 7 on pages 6–7, asks for one English `REPORT.pdf` with about 4–5 pages of main content, excluding the cover, references and brief appendices. Keep the whole PDF within the team's 20-page limit. The [README](../../README.md#demo-on-two-devices) holds the run commands. Use report space to explain design choices, verification and limitations.

| Section | What to review before submission |
| --- | --- |
| 1. System Architecture | Use one labeled diagram with the TUI, Gin, the HTTP/TCP/UDP/WebSocket paths, the separate gRPC catalog process and both SQLite owners. Show the placement of clients and services for the two-device demo. Match the labels and ports to the final run. |
| 2. Why These Technologies? | For **every** protocol, follow **Feature → Data flow → Why this technology → What can fail**. Tie the reason and failure behavior to what the code does. For TCP in particular, explain why the second client is eligible, why the byte stream needs a newline and what the TUI does after a disconnect. |
| 3. Implementation & Verification | Use two sequence or annotated diagrams. Use the progress save and TCP update for one, and the chapter release through gRPC and UDP for the other. Include a compact table with setup, expected result and actual observation from normal, negative, failure and multi-client cases. Cite the test or command that reproduces each result. |
| 4. Challenges, Reflection & Contributions | Explain a real defect through symptom, cause, fix and retest. Record known limits, An's and Vy's specific contributions, and three meaningful AI uses including a verification or review decision. The concurrent edit defect below has a reproducible failure and retest. |

The results below describe An's tests. During the group rehearsal, record the client and server placement and actual observations from both devices. Use the real run for UDP delivery, catalog outage and recovery, and cross-device TCP and WebSocket verification.

### Section 2 material for An's protocols

- **HTTP** serves accounts and library operations. The TUI sends JSON to Gin, which verifies the JWT on protected routes and reads or writes SQLite. The request and response give the TUI an immediate status and the saved record. Invalid credentials return `401`, and chapter validation rejects invalid input before a write.
- **TCP** keeps the second TUI's reading progress current. After SQLite commits, the backend sends the new chapter and version only to other sessions with the same verified account. A persistent connection lets the backend push progress when it changes. TCP carries bytes, so the newline separates JSON frames even when reads split or combine writes. On disconnect, the TUI reconnects and reloads the latest library over HTTP.
- **WebSocket** carries room discussion. The authenticated TUI upgrades its HTTP connection, joins a room and sends JSON messages. The hub checks membership before delivering each message to that room. The two-way connection lets a user receive chat while typing. A send to an unjoined room returns an error, and a disconnected client is removed from the room hub.

## Code paths to explain

1. `internal/backend/accounts/http.go` validates registration, hashes the password with bcrypt and saves it with a parameterized SQL statement. On login it checks the hash and calls `Tokens.Issue`. A signed JWT identifies both the account and this login session. HTTP, TCP and WebSocket all verify it before accepting protected work.
2. `internal/backend/library/http.go` gets the account ID from the token. It validates the requested chapter with the catalog adapter, then calls the SQLite update in `library/store.go`. That transaction increments the version, returns the saved row and commits. Only then does the handler publish a TCP event. A rejected write leaves the row and other TUI sessions unchanged.
3. `internal/backend/progress/hub.go` reads one newline-terminated JSON authentication frame. TCP can split a line across several reads, so the reader waits for the delimiter. The hub queues events for the same user ID and a different session ID. Each socket has one writer. A slow or closed socket is removed, and the TUI reloads the saved state after reconnecting.
4. `internal/backend/chat/hub.go` records a WebSocket's chosen room. It checks that room on every send and takes the sender name from the login, then queues a copy for sockets in the same room. The reader removes a disconnected member. WebSocket upgrades and frames suit interactive chat. TCP serves the independently assessed progress workflow.
5. In `internal/tui`, background workers handle HTTP and socket I/O. Widget changes go through `QueueUpdateDraw`. My Library uses the saved record's version to keep a late HTTP response or TCP message from replacing newer progress. When the user leaves a login, its context closes the network connections.

## Reproduce the checks

```powershell
go test -count=1 -v ./internal/backend/bootstrap
```

The tests use real local HTTP, TCP and WebSocket traffic with a test-only catalog fixture. For the joint demo, pass Vy's real gRPC client to `bootstrap.New` in `cmd/backend` and run both processes.

| Check | Observed result |
| --- | --- |
| `TestAccountsLibraryAndRestart` | The correct password logged in and the wrong password returned `401`. Duplicate registration returned `409`. Another account received an empty library. Chapter 30, status `completed` and version 3 survived reopening the database. With the catalog offline, a chapter change returned `503` and a status-only update preserved chapter 30. |
| `TestProgressReachesOnlySiblingTCP` | Auth sent in two pieces was accepted. The sibling got chapter 7 and version 2. The originating session and other account received no event. A chapter above the test catalog total produced `400` and no event. |
| `TestWebSocketRoomsAndDisconnect` | A message reached both users in `general`, with Alice as the verified sender. The separate room saw its own message. Chat still delivered after Bob disconnected. |
| `TestLibraryScreenDisplaysTCPPush` | The `tview` list and idle form changed from chapter 0 to chapter 12. Enter opened the form. A chapter-only save preserved another session's status change, and a status-only save preserved another session's chapter change. After interrupting TCP, the reconnected screen recovered the saved chapter. |
| `TestLibraryScreenAddsAndSwitchesAccounts` | The discovery callback added a manga to Alice's library. Switching to Bob closed Alice's TCP connection, and Bob's screen showed an empty library even after Alice saved another chapter. |
| `TestAccountScreenRegistersAndLogsIn` | Keyboard input submitted the registration and login forms over HTTP. The login callback supplied a JWT accepted by the backend. |
| `TestChatScreenJoinsAndExchangesMessages` | Keyboard input joined `general` and sent a message to Bob over WebSocket. Bob's reply appeared in the discussion transcript. |
| `TestExpiredLoginCannotKeepSocketsOpen` | The backend closed the already-open TCP and WebSocket sessions at expiry. HTTP and WebSocket upgrades returned `401`, and a fresh TCP connection received `auth_rejected` for the expired token. |
| `TestConcurrentProgressKeepsIncreasingVersions` | Two simultaneous saves returned versions 2 and 3. The stored row matched version 3. |

The standalone backend build also started twice with the same SQLite file on Windows. The account authenticated on the second start and the available HTTP routes responded. A manga add returned the planned `503` while the catalog adapter was absent.

## Framing defect and retest

A default `bufio.Scanner` treats a final JSON fragment followed by EOF as a line even when it has no newline. A small Go probe reproduced this with `{"type":"auth"}` and confirmed that `ReadSlice('\n')` returned EOF for the same input. The TCP code now requires the newline before decoding. `TestProgressReachesOnlySiblingTCP` verifies that an auth frame with no newline receives no acknowledgment while a valid frame split across writes succeeds.

A repeat audit reproduced two additional issues. The library list showed chapter 12 from TCP while the idle form still showed chapter 0. The form now takes the selected row's value while idle and preserves input while the form has keyboard focus. The socket expiry test also showed both transports staying open past the token expiry while HTTP returned `401`. Both socket readers now use the verified expiration timestamp as a read deadline and clean up at expiration.

The concurrent edit check reproduced a lost status update. One TUI opened the progress form, a second session set the manga to `reading`, and the first TUI saved only a new chapter. Sending both fields changed the status back to `plan-to-read`. The form now compares its inputs with the values loaded when editing began and sends only the edited fields. SQLite keeps the other field. The retest covers both chapter-only and status-only edits.

## AI use record for the report

1. AI helped map An's and Vy's package boundaries and design the small catalog interface used by the library.
2. AI assisted with the HTTP handlers, SQLite schema, the socket hubs and the three `tview` screens. The integration tests exercised those code paths using real loopback sockets.
3. AI reviewed the first TCP reader and identified the EOF framing case. The probe compared the two Go readers and a regression test verified the selected fix.
