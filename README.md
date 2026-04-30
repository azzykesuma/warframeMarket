# Warframe Market gRPC API

Go gRPC wrapper for the Warframe Market API v2. The server exposes item, order, and recent transaction operations through the `WarframeMarketWatcher` service and forwards requests to `https://api.warframe.market/v2/` by default.

## Project Structure

- `cmd/server/` - gRPC server, Warframe Market HTTP client, request validation, and DTO mapping.
- `cmd/client/` - sample client that connects to `localhost:50051` and calls public endpoints.
- `api/proto/watcher.proto` - service contract and protobuf message definitions.
- `api/proto/*.pb.go` - generated Go bindings for protobuf and gRPC.

## Requirements

- Go `1.25.3`
- `protoc`
- `protoc-gen-go`
- `protoc-gen-go-grpc`

## Configuration

Create a local `.env` file or set environment variables in your shell:

```env
BASE_URL=https://api.warframe.market/v2/
LANGUAGE=en
JWT_TOKEN=your-warframe-market-token
```

`BASE_URL` and `LANGUAGE` are optional. `JWT_TOKEN` is required only for authenticated order endpoints such as `GetMyOrders`, `CreateOrder`, `UpdateOrder`, and `DeleteOrder`.

## Run Locally

Start the server:

```powershell
go run ./cmd/server
```

In another terminal, run the sample client:

```powershell
go run ./cmd/client
```

Run both in separate PowerShell windows:

```powershell
Start-Process powershell -ArgumentList '-NoExit','-Command','go run ./cmd/server'
Start-Process powershell -ArgumentList '-NoExit','-Command','Start-Sleep -Seconds 2; go run ./cmd/client'
```

## API Surface

Implemented unary RPCs include:

- `ListItems`
- `GetItemById`
- `GetItemBySlug`
- `GetItemOrders`
- `GetTopItemOrders`
- `GetRecentTransactions`
- `GetOrderDetail`
- `GetMyOrders`
- `CreateOrder`
- `UpdateOrder`
- `DeleteOrder`

`WatchItem` and `WatchAllItem` are defined in the proto but currently return `Unimplemented`.

## Development

Run tests:

```powershell
go test ./...
```

Build binaries:

```powershell
go build -o server.exe ./cmd/server
go build -o main.exe ./cmd/client
```

After editing `api/proto/watcher.proto`, regenerate bindings:

```powershell
protoc --go_out=paths=source_relative:. --go-grpc_out=paths=source_relative:. api/proto/watcher.proto
```

Format Go files before committing:

```powershell
gofmt -w cmd\server cmd\client
```
