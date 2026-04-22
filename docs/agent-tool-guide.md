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
- Devices use their **ADB serial number** as the device ID (the same value from `adb devices`)
- The `device_id` in OpenFakeGPS and the `udid` in Appium are the same ADB serial
- Verify devices are available: `GET /devices` should return at least one device with `"status": "idle"`

## Device Setup via ADB

Install and launch the agent on all connected devices in one go:

```bash
for SERIAL in $(adb devices | tail -n +2 | awk '{print $1}'); do
  adb -s $SERIAL install -r android/app/build/outputs/apk/debug/app-debug.apk
  adb -s $SERIAL shell pm grant com.openfakegps.agent android.permission.ACCESS_FINE_LOCATION
  adb -s $SERIAL shell pm grant com.openfakegps.agent android.permission.ACCESS_COARSE_LOCATION
  adb -s $SERIAL shell pm grant com.openfakegps.agent android.permission.READ_PHONE_STATE
  adb -s $SERIAL shell am start -n com.openfakegps.agent/.MainActivity --es device_serial $SERIAL
done
```

The `--es device_serial <SERIAL>` flag tells the agent to use the ADB serial as its device ID. This means the `device_id` in the API matches the serial from `adb devices`, making it easy to target specific devices in automation scripts.

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

**Manual assign** (use the ADB serial as device_id):
```bash
curl -X POST http://localhost:3000/api/v1/assignments \
  -H "Content-Type: application/json" \
  -d '{"sim_id": "<simulation_id>", "device_id": "<adb_serial>"}'
```

**Response:** `{"status": "assigned", "sim_id": "...", "device_id": "..."}`

### Step 3: Start the simulation

```bash
curl -X POST http://localhost:3000/api/v1/simulations/<simulation_id>/start
```

### Step 4: Monitor progress

```bash
curl -s http://localhost:3000/api/v1/simulations/<simulation_id> | jq '{state, progress}'
```

- `progress`: 0.0 to 1.0 (fraction of route completed)
- `state`: `created` | `running` | `paused` | `stopped`

Poll every 2-5 seconds to track completion.

### Step 5: Stop when done

```bash
curl -X POST http://localhost:3000/api/v1/simulations/<simulation_id>/stop
```

## Using Stops for Realistic Driving

The `stops` array pauses movement at specific waypoints to simulate traffic lights, stop signs, and congestion. This makes trips look like real driving instead of constant-speed movement.

Each stop has:

| Field | Type | Description |
|-------|------|-------------|
| `waypoint_index` | int | 0-based index into the route/polyline waypoints where the vehicle should stop |
| `duration` | string | How long to hold position (Go duration format: `"10s"`, `"1m30s"`, `"500ms"`) |

When the vehicle reaches a stop waypoint:
- It **brakes realistically** as it approaches (gradual deceleration)
- It **holds position** at the exact waypoint with speed = 0 for the specified duration
- It **resumes movement** automatically after the duration elapses

**Example — a trip with stops simulating traffic lights:**

```bash
curl -X POST http://localhost:3000/api/v1/simulations \
  -H "Content-Type: application/json" \
  -d '{
    "polyline": "b}ejCxjtgGjAhAmE`BuC?sBhAwDrFi@s@~@mCpAmBmDp@",
    "speed_kmh": 40,
    "update_interval": "1s",
    "noise_meters": 2.0,
    "stops": [
      {"waypoint_index": 2, "duration": "15s"},
      {"waypoint_index": 5, "duration": "8s"},
      {"waypoint_index": 9, "duration": "25s"}
    ]
  }'
```

Recommended stop durations:
- **Traffic lights**: 10-30 seconds, place every 3-8 waypoints in city routes
- **Stop signs**: 3-5 seconds
- **Congestion/slow zones**: 30-60 seconds, or multiple short stops close together
- **Pickup/dropoff wait**: 30-120 seconds
- Vary durations randomly to avoid patterns that look scripted

## Workflow: Simulating a Ride-Hailing Trip (e.g. Uber)

### Phase 1: Set the pickup location

Create a minimal simulation at the pickup point so the device GPS stabilizes there:

```bash
curl -X POST http://localhost:3000/api/v1/simulations \
  -H "Content-Type: application/json" \
  -d '{
    "route": [
      {"lat": -23.5505, "lon": -46.6333},
      {"lat": -23.5506, "lon": -46.6334}
    ],
    "speed_kmh": 1,
    "update_interval": "1s",
    "noise_meters": 2.0
  }'
```

Assign, start, and wait 3-5 seconds for the GPS to stabilize before interacting with the ride-hailing app:

```bash
curl -X POST http://localhost:3000/api/v1/assignments \
  -H "Content-Type: application/json" \
  -d '{"sim_id": "<SIM_ID>", "device_id": "<ADB_SERIAL>"}'

curl -X POST http://localhost:3000/api/v1/simulations/<SIM_ID>/start
```

### Phase 2: Request the ride

Use your UI automation tool (e.g. Appium) to interact with the app. The device's `udid` in Appium is the same ADB serial used as `device_id` in OpenFakeGPS.

- Wait for the map to load and detect the current (fake) location
- Enter the destination and request the ride
- Wait for a driver to be matched

### Phase 3: Simulate the trip

Once the driver arrives, stop the pickup simulation and create a new one for the trip route:

```bash
curl -X POST http://localhost:3000/api/v1/simulations/<SIM_ID>/stop
```

Create the trip simulation using a Google Directions polyline for realistic road-following movement, with stops for traffic:

```bash
curl -X POST http://localhost:3000/api/v1/simulations \
  -H "Content-Type: application/json" \
  -d '{
    "polyline": "<GOOGLE_DIRECTIONS_POLYLINE>",
    "speed_kmh": 35,
    "update_interval": "1s",
    "noise_meters": 2.0,
    "stops": [
      {"waypoint_index": 3, "duration": "12s"},
      {"waypoint_index": 8, "duration": "20s"},
      {"waypoint_index": 15, "duration": "8s"}
    ]
  }'
```

Assign and start:

```bash
curl -X POST http://localhost:3000/api/v1/assignments \
  -H "Content-Type: application/json" \
  -d '{"sim_id": "<TRIP_SIM_ID>", "device_id": "<ADB_SERIAL>"}'

curl -X POST http://localhost:3000/api/v1/simulations/<TRIP_SIM_ID>/start
```

Monitor progress until complete:

```bash
curl -s http://localhost:3000/api/v1/simulations/<TRIP_SIM_ID> | jq '{state, progress}'
```

### Phase 4: Complete the ride

When `progress >= 1.0` or `state == "stopped"`, the trip is done. Use your UI automation tool to handle the post-ride screen, then stop the simulation:

```bash
curl -X POST http://localhost:3000/api/v1/simulations/<TRIP_SIM_ID>/stop
```

## Getting a Route Polyline

To get a realistic driving route between two points, use the Google Directions API:

```bash
curl "https://maps.googleapis.com/maps/api/directions/json?origin=-23.5505,-46.6333&destination=-23.5625,-46.6543&key=<GOOGLE_API_KEY>" \
  | jq -r '.routes[0].overview_polyline.points'
```

Pass the resulting encoded polyline string directly to the `"polyline"` field.

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
| `POST` | `/simulations` | Create simulation with route/polyline + optional stops |
| `GET` | `/simulations` | List all simulations |
| `GET` | `/simulations/{id}` | Get simulation status, position, and progress |
| `POST` | `/simulations/{id}/start` | Start simulation |
| `POST` | `/simulations/{id}/pause` | Pause simulation |
| `POST` | `/simulations/{id}/resume` | Resume paused simulation |
| `POST` | `/simulations/{id}/stop` | Stop simulation (terminal, create new one for next trip) |
| `POST` | `/simulations/{id}/position` | Set position directly (lat, lon, speed, heading) |
| `GET` | `/devices` | List connected devices |
| `GET` | `/devices/{id}` | Get device details |
| `POST` | `/assignments` | Manual assign: `{"sim_id", "device_id"}` |
| `POST` | `/assignments/auto` | Auto-assign to first idle device: `{"sim_id"}` |

## Error Handling

All errors return JSON: `{"error": "description"}`

| Status | Meaning |
|--------|---------|
| 400 | Bad request: invalid JSON, missing fields, invalid state transition, mutually exclusive params |
| 404 | Simulation or device not found |
| 500 | Internal server error |

## Error Recovery

- If the app shows "GPS signal lost": check the simulation is running — `curl -s http://localhost:3000/api/v1/simulations/<SIM_ID> | jq '{state}'` should show `"running"`
- If the device disconnects from OpenFakeGPS: `adb -s <SERIAL> shell am start -S -n com.openfakegps.agent/.MainActivity --es device_serial <SERIAL>`
- If assignment fails with "no idle devices": check `curl -s http://localhost:3000/api/v1/devices | jq '.[] | {agent_id, status}'` — stop any running simulation first

## Tips for Agent Integration

- **Use polyline**: Easier to pass a single string than an array of coordinates. Google Directions API returns encoded polylines directly in `overview_polyline.points`.
- **Use auto-assign**: `POST /assignments/auto` is simpler than listing devices and picking one manually.
- **Use `speed_kmh`**: More intuitive than m/s for most use cases. Typical city driving: 30-50 km/h.
- **Add stops**: Routes without any pauses look robotic. Add 2-5 stops per trip for realism.
- **Simulation is one-shot**: Once stopped, a simulation cannot be restarted. Create a new one for each trip.
- **Always stop before creating a new simulation** for the same device. One simulation per device at a time.
- **Device IDs are ADB serials**: The `device_id` / `agent_id` in the API matches the serial from `adb devices`. Same value used as `udid` in Appium.
- **GPS noise**: Set `noise_meters` to 1-3 for realistic GPS jitter. 0 is suspiciously perfect.
- **Wait for GPS to stabilize**: Give 3-5 seconds after starting a simulation before interacting with apps that read location.
- **Multi-device**: Each device gets its own simulation. Create, assign, and start independently using different ADB serials. All simulations run concurrently on the backend.
