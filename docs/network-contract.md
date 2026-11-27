# MangaHub network contract

These are the interfaces between An's backend and screens and Vy's catalog, UDP and TUI shell. The JSON behavior follows the [development plan](../MANGAHUB_DEVELOPMENT_PLAN.md).

## Backend integration

- Vy's catalog adapter implements `library.Catalog` with `GetManga(context.Context, string) (contract.Manga, error)`. The input string is the stable manga ID. Apply the RPC timeout in the adapter. Map invalid IDs, missing records and unavailable or timed-out RPCs to `contract.ErrInvalidCatalogInput`, `contract.ErrMangaNotFound` and `contract.ErrCatalogUnavailable` respectively. Wrapped errors are accepted.
- An passes the adapter as the third argument of `bootstrap.New(db, tokens, catalog)` in `cmd/backend/main.go`. Give An the discovery and release route registration functions and the UDP start/stop functions. An mounts them on `app.Router` and wires the UDP lifecycle. The backend already serves `/api/health`, auth, library and `/ws`.
- Use `tokens.RequireUser()` for a protected Gin route, `accounts.User(c)` inside that route, and `tokens.Verify(token)` for UDP. The verified identity has a user ID, a new session ID for every login and JWT expiration time in `ExpiresAt`. The library looks up the manga through gRPC for new entries and chapter changes. A status-only update needs only backend SQLite.
- Errors use `{"error":{"code":"code","message":"Readable message."}}`. Manga records share `contract.Manga`. Vy owns the Protobuf definitions and generated code. Use the scheduled UDP `subscribe`, `unsubscribe`, `subscribed` and `new_chapter` messages. Send each message in one datagram and use the sender's actual source address for registration.

## Using An's TUI screens

- Create `api := contract.NewHTTPClient("http://" + apiAddr)`. `api.Do` handles JSON and the Bearer token for any Gin request. Use the application context with `accounts.NewScreen(app, appContext, api, onLogin)`. The callback receives the JWT and runs on the TUI event loop.
- Create a fresh `context.WithCancel` for each login. Pass it to `library.NewScreen(app, sessionContext, api, token, onTCPStatus)` and `chat.NewScreen(app, sessionContext, "ws://"+apiAddr+"/ws", token, onWSStatus)`. Mount their `View()` values as pages. Status callbacks are optional and run on the TUI event loop.
- Start `go libraryScreen.RunTCP(tcpAddr)` once per login. It loads the current library, listens for progress and reloads once after a connection is established or restored. From the discovery screen, call `libraryScreen.Add(selectedManga, onResult)` on the TUI event loop. `onResult(error)` receives the result on that loop too. A successful add updates My Library, using the title supplied in `selectedManga` and falling back to IDs on a later login.
- Cancel the session context on logout, then replace its pages. Cancel the app context before leaving the TUI. Build new screen instances for the next login. My Library uses Enter or Tab to edit and Esc to return to the list. Chat connects when the user selects Join room.

## Address configuration for two devices

In the integrated app, the backend uses `API_ADDR`, `TCP_ADDR` and `UDP_ADDR` as listen addresses. The TUI reads the same environment keys as remote addresses in its own process. For the two-device demo, set them to An's reachable LAN address on both TUI instances. Derive the WebSocket URL from `API_ADDR`. Use `UDP_ADDR` as the registration destination and bind a separate local UDP socket with an OS-assigned port per TUI. `CATALOG_ADDR` is used by the backend and catalog processes on An's device. See the [two-device run guide](../README.md#demo-on-two-devices).

## Messages owned by An

TCP starts with `{"type":"auth","token":"<jwt>"}` and replies `{"type":"auth_ok"}`. An invalid or expired token receives `{"type":"auth_rejected"}`. Each object ends with a newline. An example event is `{"type":"progress_updated","manga_id":"one-piece","chapter":12,"status":"reading","updated_at":"2026-11-26T10:00:00Z","version":2}`. Only other logins of the same account receive it. TCP and WebSocket sessions close when their JWT expires. The user then signs in again.

WebSocket uses `GET /ws` with `Authorization: Bearer <jwt>`. Send `{"type":"join","room":"general"}` and wait for `{"type":"joined","room":"general"}`. Send `{"type":"chat","room":"general","text":"Hello"}`. The hub fills in `sender` using the login token and returns chat to that room. An invalid action returns a message with `type` equal to `error` and a readable `text` field.
