# Repository Guidelines

## Project Structure & Module Organization
This is a Go gRPC service for Warframe Market integration. The module root is `github.com/azzykesuma/warframeMarket`.

- `cmd/server/` contains the gRPC server, Warframe Market HTTP client logic, and response DTOs.
- `cmd/client/` contains a small local client used to exercise the server on `localhost:50051`.
- `api/proto/watcher.proto` is the source contract for the service.
- `api/proto/*.pb.go` are generated from the proto file; update them whenever `watcher.proto` changes.
- `.env` provides local configuration such as `JWT_TOKEN` and optional `BASE_URL`; do not commit real credentials.

## Build, Test, and Development Commands
- `go run ./cmd/server` starts the gRPC server on port `50051`.
- `go run ./cmd/client` runs the sample client against the local server.
- `go test ./...` compiles and tests all packages.
- `go build -o server.exe ./cmd/server` builds the server binary.
- `go build -o main.exe ./cmd/client` builds the sample client binary.
- `protoc --go_out=. --go-grpc_out=. api/proto/watcher.proto` regenerates protobuf and gRPC bindings after contract changes.

## Coding Style & Naming Conventions
Use standard Go formatting: run `gofmt` on edited `.go` files before committing. Keep package names short and lowercase. Exported Go identifiers should use PascalCase only when they are part of the package API or required for JSON/protobuf mapping; otherwise prefer unexported camelCase. Keep external API DTOs in `cmd/server/types.go` and gRPC behavior in `cmd/server/main.go` unless the server grows enough to justify new files.

## Testing Guidelines
There are currently no dedicated test files. Add table-driven `*_test.go` tests near the code under test, especially for request validation, Warframe Market response mapping, and error handling. For HTTP-dependent behavior, prefer `httptest.Server` over live API calls. Run `go test ./...` before opening a change.

## Commit & Pull Request Guidelines
This checkout has no local Git history, so follow a conservative convention: use short imperative commit subjects such as `add order detail mapping` or `fix client nil order handling`. Pull requests should describe the gRPC/API behavior changed, list validation commands, mention any `.proto` regeneration, and include screenshots or logs only when they clarify runtime behavior.

## Security & Configuration Tips
Keep `JWT_TOKEN` in `.env` or your shell environment, never in source. Avoid logging authorization headers or full authenticated responses. When adding new Warframe Market endpoints, validate required request fields before making outbound calls and return appropriate gRPC status codes.
