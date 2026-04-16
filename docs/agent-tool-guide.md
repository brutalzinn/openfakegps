# OpenFakeGPS - Agent Tool Guide

Use this document when integrating OpenFakeGPS as a tool for AI agents or automation scripts that need to simulate GPS movement on Android devices.

## What This Tool Does

OpenFakeGPS injects fake GPS coordinates into real Android devices. You control it via REST API: create a simulation with a route, assign it to a connected device, start it, and the device's GPS will follow that route at the configured speed. Any app on the device (ride-hailing, delivery, maps) will see the simulated location.

## Base URL

```
http://localhost:3000/api/v1
```

## Prerequisites

- Backend running (`go run ./cmd/server` or `docker compose up`)
- At least one Android device connected via the agent app, with mock location enabled
- Verify devices are available: `GET /devices` should return at least one device with `"status": "idle"`

## Quick Start (Minimum Steps)

### Step 1: Create a simulation

Use **either** a polyline (recommended for agents) or explicit waypoints.

**With polyline (preferred):**
```bash
curl -X POST http://localhost:3000/api/v1/simulations \
  -H "Content-Type: application/json" \
  -d '{
    "polyline": "<encoded_polyline_string>",
    "speed_kmh": 40,
    "update_interval": "1s",
    "noise_meters": 2.0
  }'
```

**With waypoints:**
```bash
curl -X POST http://localhost:3000/api/v1/simulations \
  -H "Content-Type: application/json" \
  -d '{
    "route": [
      {"lat": -23.5505, "lon": -46.6333},
      {"lat": -23.5515, "lon": -46.6343},
      {"lat": -23.5525, "lon": -46.6353}
    ],
    "speed_kmh": 40,
    "update_interval": "1s",
    "noise_meters": 2.0
  }'
```

**Response:** `{"id": "<simulation_id>"}`

### Step 2: Assign to a device

**Auto-assign** (picks first idle device):
```bash
curl -X POST http://localhost:3000/api/v1/assignments/auto \
  -H "Content-Type: application/json" \
  -d '{"sim_id": "<simulation_id>"}'
```

**Manual assign** (if you know the device):
```bash
curl -X POST http://localhost:3000/api/v1/assignments \
  -H "Content-Type: application/json" \
  -d '{"sim_id": "<simulation_id>", "device_id": "<agent_id>"}'
```

**Response:** `{"status": "assigned", "sim_id": "...", "device_id": "..."}`

### Step 3: Start the simulation

```bash
curl -X POST http://localhost:3000/api/v1/simulations/<simulation_id>/start
```

### Step 4: Monitor progress

```bash
curl http://localhost:3000/api/v1/simulations/<simulation_id>
```

**Response:**
```json
{
  "id": "...",
  "state": "running",
  "device_id": "...",
  "current_pos": {
    "Lat": -23.5510,
    "Lon": -23.6338,
    "Speed": 11.1,
    "Bearing": 215.3,
    "Accuracy": 2.1,
    "Altitude": 0,
    "Timestamp": "2026-04-16T12:00:00Z"
  },
  "progress": 0.45
}
```

- `progress`: 0.0 to 1.0 (fraction of route completed)
- `state`: `created` | `running` | `paused` | `stopped`
- `Speed` is in m/s

### Step 5: Stop when done

```bash
curl -X POST http://localhost:3000/api/v1/simulations/<simulation_id>/stop
```

## Parameter Reference

### Create Simulation (`POST /simulations`)

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `route` | array | One of route/polyline | Array of `{"lat": float, "lon": float}` waypoints (min 2) |
| `polyline` | string | One of route/polyline | Google encoded polyline string |
| `speed_kmh` | float | No | Speed in km/h (default ~50). Mutually exclusive with `speed_mps` |
| `speed_mps` | float | No | Speed in m/s (default 13.8). Mutually exclusive with `speed_kmh` |
| `update_interval` | string | No | Go duration format, e.g. `"1s"`, `"500ms"` (default `"1s"`) |
| `noise_meters` | float | No | GPS noise radius in meters (default 2.0) |
| `stops` | array | No | Planned stops: `[{"waypoint_index": int, "duration": "30s"}]` |

### Simulation States

```
created -> running -> paused -> running (resumed)
                  \-> stopped (terminal, cannot restart)
```

- A stopped simulation cannot be restarted. Create a new one.
- One simulation per device at a time.

## All Endpoints

| Method | Path | Description |
|--------|------|-------------|
| `POST` | `/simulations` | Create simulation with route or polyline |
| `GET` | `/simulations` | List all simulations |
| `GET` | `/simulations/{id}` | Get simulation status and current position |
| `POST` | `/simulations/{id}/start` | Start simulation |
| `POST` | `/simulations/{id}/pause` | Pause simulation |
| `POST` | `/simulations/{id}/resume` | Resume paused simulation |
| `POST` | `/simulations/{id}/stop` | Stop simulation (terminal) |
| `GET` | `/devices` | List connected devices |
| `GET` | `/devices/{id}` | Get device details |
| `POST` | `/assignments` | Manual assign: `{"sim_id", "device_id"}` |
| `POST` | `/assignments/auto` | Auto-assign to idle device: `{"sim_id"}` |

## Error Handling

All errors return JSON: `{"error": "description"}`

| Status | Meaning |
|--------|---------|
| 400 | Bad request: invalid JSON, missing fields, invalid state transition, mutually exclusive params |
| 404 | Simulation or device not found |
| 500 | Internal server error |

## Tips for Agent Integration

- **Use polyline**: Easier to pass a single string than an array of coordinates. Google Directions API returns encoded polylines directly in `overview_polyline.points`.
- **Use auto-assign**: `POST /assignments/auto` is simpler than listing devices and picking one manually.
- **Use `speed_kmh`**: More intuitive than m/s for most use cases. Typical city driving: 30-50 km/h.
- **Poll progress**: `GET /simulations/{id}` returns `progress` (0.0-1.0). Poll every 2-5 seconds to track completion.
- **Simulation is one-shot**: Once stopped, a simulation cannot be restarted. Create a new one for each trip.
- **Check device availability first**: `GET /devices` — if no devices have `"status": "idle"`, assignment will fail.
- **GPS noise**: Set `noise_meters` to 1-5 for realistic GPS jitter. Set to 0 for exact coordinates.

## Example: Full Automated Flow

```python
import requests
import time

BASE = "http://localhost:3000/api/v1"

# 1. Create simulation from polyline
sim = requests.post(f"{BASE}/simulations", json={
    "polyline": "b}ejCxjtgGjAhAmE`BuC?sBhAwDrFi@s@~@mCpAmBmDp@",
    "speed_kmh": 40,
    "update_interval": "1s",
    "noise_meters": 2.0
}).json()
sim_id = sim["id"]

# 2. Auto-assign to available device
assignment = requests.post(f"{BASE}/assignments/auto", json={
    "sim_id": sim_id
}).json()
device_id = assignment["device_id"]

# 3. Start
requests.post(f"{BASE}/simulations/{sim_id}/start")

# 4. Wait for completion
while True:
    status = requests.get(f"{BASE}/simulations/{sim_id}").json()
    print(f"Progress: {status['progress']:.1%} | State: {status['state']}")
    if status["state"] == "stopped" or status["progress"] >= 1.0:
        break
    time.sleep(3)

# 5. Stop (if not already stopped)
if status["state"] != "stopped":
    requests.post(f"{BASE}/simulations/{sim_id}/stop")
```
