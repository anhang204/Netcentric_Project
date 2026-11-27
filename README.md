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

## Demo on two devices

After integration, connect An's and Vy's devices to the same private Wi-Fi or hotspot. On An's device, run `ipconfig` and find the IPv4 address of the adapter on that network. Replace `192.168.1.20` below with that address.

Start the catalog on An's device at `127.0.0.1:9092` with `go run ./cmd/catalog`. In a second terminal on An's device, start the backend with these settings:

```powershell
$demoHost = '192.168.1.20'
$env:API_ADDR = "${demoHost}:8080"
$env:TCP_ADDR = "${demoHost}:9090"
$env:UDP_ADDR = "${demoHost}:9091"
$env:CATALOG_ADDR = '127.0.0.1:9092'
$env:JWT_SECRET = [guid]::NewGuid().ToString('N')
$env:ADMIN_DEMO_KEY = [guid]::NewGuid().ToString('N')
go run ./cmd/backend
```

Open a TUI terminal on each device. Use An's address in both terminals:

```powershell
$demoHost = '192.168.1.20'
$env:API_ADDR = "${demoHost}:8080"
$env:TCP_ADDR = "${demoHost}:9090"
$env:UDP_ADDR = "${demoHost}:9091"
Invoke-RestMethod "http://${demoHost}:8080/api/health"
go run ./cmd/tui
```

Check `/api/health` from Vy's device before the demo. If the request fails, check the IP, whether the devices can reach each other on the network, and whether inbound TCP `8080` and `9090` and UDP `9091` are allowed on An's private network. Catalog gRPC stays on An's device.

1. Log in as the same synthetic account on both devices. Find and add a manga, change its chapter on An's TUI, and show the update on Vy's TUI.
2. Subscribe to UDP alerts on both devices and trigger a release from the backend's admin/demo flow. Show the notification on each device.
3. Log Vy into a different synthetic account and join the same chat room from both devices. Exchange messages. Use a third TUI session on either device for the separate-account and separate-room checks.

## Verify An's work

```powershell
go test -count=1 ./...
go vet ./...
go test -race -count=1 ./...
```

The bootstrap tests open real loopback HTTP, TCP and WebSocket listeners and temporary SQLite databases. Their catalog is a test fixture. The backend on An's branch starts accounts, library listing, TCP and chat. Manga additions and chapter changes return `503` until Vy's catalog adapter is passed at startup. The integrated TUI executable and the catalog/UDP features are delivered from Vy's branch.
