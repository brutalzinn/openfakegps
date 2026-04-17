# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

OpenFakeGPS is a cross-platform GPS simulation system for automated ride-hailing testing. Phase 1 targets Android only. The system simulates realistic GPS movement on real Android devices, controlled via REST API with real-time location streaming over gRPC.

## Architecture

Three main components communicate in this pattern:

```
Test Scripts -> REST API (Go) -> Simulation Engine -> gRPC Stream -> Android Agent -> Mock GPS
```

- **Go Backend** (`backend/`): Simulation engine, device orchestration, gRPC server, REST API
- **Android Agent** (`android/`): Kotlin foreground service that receives gRPC location updates and injects them via Android Mock Location Provider
- **Proto Definitions** (`proto/`): Shared gRPC/protobuf contract used by both backend and Android

### Backend Internal Structure

- `internal/simulation/` - Simulation engine: manages lifecycle (create/start/pause/resume/stop), movement interpolation along routes, GPS noise injection. Each simulation runs in its own goroutine with context-based cancellation.
- `internal/orchestration/` - Device registry (thread-safe map of connected agents) and orchestrator (assigns simulations to devices, handles connect/disconnect).
- `internal/grpcserver/` - Bidirectional streaming gRPC server. Agents register on first message, then receive location updates. `streamWrapper` implements `orchestration.ServerStream` to bridge gRPC streams into the registry.
- `internal/api/` - REST API using Go 1.22 `ServeMux` with method+path patterns. Control plane only (no streaming).
- `pkg/geo/` - Haversine distance, bearing, great-circle interpolation utilities.
- `proto/fakegps/v1/` - Hand-written proto Go types and gRPC service registration (replace with protoc-generated code when available).

### Key Wiring (main.go)

The location callback connects simulation ticks to gRPC delivery: engine tick -> callback looks up device stream in registry -> sends LocationUpdate over gRPC. This is wired in `cmd/server/main.go`.

### Android Agent Structure

- `service/LocationService.kt` - Foreground service with WakeLock, manages AgentClient lifecycle
- `grpc/AgentClient.kt` - gRPC bidirectional streaming client with coroutines, exponential backoff reconnection
- `location/MockLocationProvider.kt` - Injects Location objects via `LocationManager.setTestProviderLocation`
- `ui/MainViewModel.kt` - Shared LiveData singleton updated from service, observed by activity
- Proto-generated classes are built from `../../../proto` via protobuf Gradle plugin

## Build & Run

### Backend (Go)

```bash
cd backend
go build ./...                          # build all
go run ./cmd/server                     # run (default: gRPC :50051, HTTP :3000)
go run ./cmd/server -grpc-port 50051 -http-port 3000  # explicit ports
go test ./...                           # run tests
go test ./internal/simulation/...       # run single package tests
go vet ./...                            # static analysis
```

### Android Agent

```bash
cd android
./gradlew assembleDebug                 # build debug APK
./gradlew installDebug                  # install on connected device
./gradlew test                          # run unit tests
```

Requires Android SDK with compileSdk 35, minSdk 26. The device must have "Allow mock locations" enabled in Developer Options.

### Docker

```bash
docker compose up --build               # run backend with Docker
```

### Proto Regeneration (when protoc is available)

```bash
protoc --go_out=backend --go-grpc_out=backend proto/fakegps/v1/fakegps.proto
```

The Android proto code is generated automatically by the protobuf Gradle plugin from the shared `proto/` directory.

## REST API

Base: `http://localhost:3000/api/v1`

Swagger UI: `http://localhost:3000/swagger`

| Method | Path | Purpose |
|--------|------|---------|
| POST | /simulations | Create simulation with route |
| GET | /simulations | List all simulations |
| GET | /simulations/{id} | Get simulation status + position |
| POST | /simulations/{id}/start | Start simulation |
| POST | /simulations/{id}/pause | Pause simulation |
| POST | /simulations/{id}/resume | Resume simulation |
| POST | /simulations/{id}/stop | Stop simulation |
| POST | /simulations/{id}/position | Set lat/lon/speed/heading manually |
| GET | /devices | List connected devices |
| GET | /devices/{id} | Get device details |
| POST | /assignments | Manual assign (sim_id + device_id) |
| POST | /assignments/auto | Auto-assign to idle device (sim_id) |

Full documentation in `docs/api.md` and `docs/grpc.md`.

## Concurrency Model

- Each simulation runs in a dedicated goroutine with `context.WithCancel`
- Device registry uses `sync.RWMutex` for concurrent reads
- gRPC stream writes are serialized per-device via `streamWrapper.mu`
- The engine's location callback is the bridge between simulation goroutines and gRPC delivery

## Constraints

- One simulation per device (initial constraint)
- Stopped simulations cannot be restarted (create a new one)
- The first gRPC message from an agent must be `RegisterRequest`
- Backend proto files are hand-written stubs; replace with protoc-generated code for production
