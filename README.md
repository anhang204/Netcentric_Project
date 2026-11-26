# MangaHub

## Prerequisites

- Go 1.25 or later and a C compiler available on `PATH` for `go-sqlite3`. Set `CGO_ENABLED=1` if your Go installation disables it.
- A terminal that supports `tview`. The integrated app uses the separate catalog process to supply manga details and chapter limits.

## Run

From the project root, use `.env.example` as the list of settings. The programs read environment variables from their process, so set the required variables in each terminal. For the backend in PowerShell:

```powershell
$env:JWT_SECRET = 'replace-with-a-local-secret-of-32-or-more-bytes'
go run ./cmd/backend
```

The default addresses are HTTP and WebSocket at `127.0.0.1:8080`, TCP at `127.0.0.1:9090`, UDP at `127.0.0.1:9091` and gRPC at `127.0.0.1:9092`. To run a second stack, set different address and database variables in that stack's terminals. Register local synthetic users such as `alice`, `bob` and `charlie` through the login/register screen or `/api/auth/register`.

After integration, start `go run ./cmd/catalog`, then `go run ./cmd/backend`, then one or more `go run ./cmd/tui` processes. See [the network contract](docs/network-contract.md) for the current backend and TUI handoffs.

## Verify An's work

```powershell
go test -count=1 ./...
go vet ./...
go test -race -count=1 ./...
```

The bootstrap tests open real loopback HTTP, TCP and WebSocket listeners and temporary SQLite databases. Their catalog is a test fixture. The backend on An's branch starts accounts, library listing, TCP and chat. Manga additions and chapter changes return `503` until Vy's catalog adapter is passed at startup. The integrated TUI executable and the catalog/UDP features are delivered from Vy's branch.
