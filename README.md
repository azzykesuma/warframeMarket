# Warframe Market Backend API

Go backend for Warframe Market API v2. The server exposes the same functionality over gRPC on `localhost:50051` and HTTP/JSON on `localhost:8080` for frontend apps.

## Project Structure

- `cmd/server/main.go` - process bootstrap for HTTP and gRPC servers.
- `cmd/server/app.go` - application service configuration and shared server state.
- `cmd/server/auth.go` - app login, logout, session tokens, and gRPC auth interceptors.
- `cmd/server/wfm_client.go` - Warframe Market HTTP client, rate limiting, and upstream error mapping.
- `cmd/server/market_handlers.go` - gRPC market and order use-case handlers.
- `cmd/server/rest.go` - HTTP/JSON routes for frontend clients.
- `cmd/server/mappers.go` and `types.go` - DTO and protobuf mapping.
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
HTTP_ADDR=:8080
APP_USERNAME=aoi umi
APP_PASSWORD=blue_sea_30
JWT_TOKEN=your-warframe-market-token
FIREBASE_PROJECT_ID=your-firebase-project-id
FIREBASE_CREDENTIALS_FILE=./firebase-service-account.json
FIRESTORE_INVENTORY_COLLECTION=inventory_items
```

`BASE_URL`, `LANGUAGE`, `HTTP_ADDR`, and `FIRESTORE_INVENTORY_COLLECTION` are optional. `APP_USERNAME` and `APP_PASSWORD` control access to this personal API. `JWT_TOKEN` is separate and is required only for authenticated Warframe Market endpoints such as `GetMyOrders`, `CreateOrder`, `UpdateOrder`, and `DeleteOrder`. Inventory CRUD uses Firestore through Firebase Admin credentials.

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

## Frontend HTTP API

Start with login:

```http
POST /api/login
Content-Type: application/json

{"username":"aoi umi","password":"blue_sea_30"}
```

Use the returned `session_token` on every later request:

```http
Authorization: Bearer <session_token>
```

The login response also includes `accessToken`, `access_token`, and `token` aliases with the same value so frontend helpers can read the token consistently.

Available HTTP routes:

- `POST /api/login`
- `POST /api/logout`
- `GET /api/items?language=en`
- `GET /api/items/id/{item_id}?language=en`
- `GET /api/items/slug/{item_slug}?language=en`
- `GET /api/items/{item_slug}/orders`
- `GET /api/items/{item_slug}/orders/top?rank=0`
- `GET /api/transactions/recent?item_name={name}`
- `GET /api/orders/{id}`
- `GET /api/orders/my`
- `POST /api/orders`
- `PUT /api/orders/{id}`
- `DELETE /api/orders/{id}`
- `GET /api/inventory`
- `POST /api/inventory`
- `GET /api/inventory/{id}`
- `PUT /api/inventory/{id}`
- `DELETE /api/inventory/{id}`
- `POST /api/inventory/bulk`

The HTTP API supports CORS preflight requests and returns JSON errors shaped like `{"success":false,"code":"Unauthenticated","error":"..."}`.

Inventory item JSON:

```json
{
  "item_id": "optional-wfm-item-id",
  "item_slug": "nikana_prime_set",
  "item_name": "Nikana Prime Set",
  "quantity": 1,
  "rank": 0,
  "subtype": "",
  "notes": "optional note",
  "acquired_at": "2026-04-30"
}
```

Bulk upload accepts:

```json
{
  "replace": true,
  "items": [
    {
      "item_slug": "nikana_prime_set",
      "item_name": "Nikana Prime Set",
      "quantity": 1
    }
  ]
}
```

## gRPC API Surface

Public auth RPCs:

- `Login`

Implemented unary RPCs include:

- `Logout`
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

Call `Login` first with `APP_USERNAME` and `APP_PASSWORD`. Use the returned token on all later calls as gRPC metadata:

```text
authorization: Bearer <session_token>
```

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
