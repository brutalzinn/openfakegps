# REST API Documentation

Base URL: `http://localhost:3000/api/v1`

Swagger UI: `http://localhost:3000/swagger`

All responses are JSON. Errors return `{"error": "message"}`.

---

## Simulation Management

### Create Simulation

```
POST /simulations
```

**Body (using route array):**
```json
{
  "route": [
    {"lat": -23.5505, "lon": -46.6333},
    {"lat": -23.5515, "lon": -46.6343},
    {"lat": -23.5525, "lon": -46.6353}
  ],
  "speed_kmh": 50,
  "update_interval": "1s",
  "noise_meters": 2.0,
  "stops": [
    {"waypoint_index": 1, "duration": "5s"}
  ]
}
```

**Body (using encoded polyline):**
```json
{
  "polyline": "b}ejCxjtgGjAhAmE`BuC?sBhAwDrFi@s@~@mCpAmBmDp@",
  "speed_kmh": 50,
  "update_interval": "1s",
  "noise_meters": 2.0
}
```

- `route`: Array of waypoints (minimum 2). Coordinates in decimal degrees. **Mutually exclusive with `polyline`**.
- `polyline`: Google encoded polyline string. Decoded internally into waypoints. See [Polyline Algorithm](https://developers.google.com/maps/documentation/utilities/polylinealgorithm). **Mutually exclusive with `route`**.
- `speed_mps` (optional): Target speed in meters/second. Default: 13.8 (~50 km/h). **Mutually exclusive with `speed_kmh`**.
- `speed_kmh` (optional): Target speed in km/h (converted to m/s internally). **Mutually exclusive with `speed_mps`**.
- `update_interval` (optional): Go duration string. Default: "1s".
- `noise_meters` (optional): GPS noise radius in meters. Default: 2.0.
- `stops` (optional): Planned stops at waypoint indices with duration.

**Response (201):**
```json
{"id": "uuid-string"}
```

### List Simulations

```
GET /simulations
```

**Response (200):**
```json
[
  {
    "id": "uuid",
    "state": "created",
    "device_id": "",
    "current_pos": {"Lat": -23.5505, "Lon": -46.6333, "Speed": 0, "Bearing": 0, "Accuracy": 0, "Altitude": 0, "Timestamp": "..."},
    "progress": 0.0
  }
]
```

### Get Simulation

```
GET /simulations/{id}
```

**Response (200):** Same shape as list item.

### Start Simulation

```
POST /simulations/{id}/start
```

**Response (200):** `{"status": "started"}`

### Pause Simulation

```
POST /simulations/{id}/pause
```

**Response (200):** `{"status": "paused"}`

### Resume Simulation

```
POST /simulations/{id}/resume
```

**Response (200):** `{"status": "resumed"}`

### Stop Simulation

```
POST /simulations/{id}/stop
```

**Response (200):** `{"status": "stopped"}`

---

## Device Management

### List Devices

```
GET /devices
```

**Response (200):**
```json
[
  {
    "agent_id": "08291JEC212687",
    "device_name": "sunfish",
    "model": "Pixel 4a",
    "capabilities": ["mock_location", "gps_simulation"],
    "status": "idle",
    "assigned_sim": "",
    "last_seen": "2026-04-20T17:20:51Z"
  }
]
```

> **Note:** The `agent_id` is the device's ADB serial number (same value from `adb devices`). Use it as `device_id` when assigning simulations.

### Get Device

```
GET /devices/{id}
```

**Response (200):** Same shape as list item.

---

## Assignment

### Manual Assignment

```
POST /assignments
```

**Body:**
```json
{
  "sim_id": "simulation-uuid",
  "device_id": "08291JEC212687"
}
```

The `device_id` is the ADB serial number of the target device.

**Response (200):**
```json
{
  "status": "assigned",
  "sim_id": "simulation-uuid",
  "device_id": "08291JEC212687"
}
```

### Auto-Assignment

```
POST /assignments/auto
```

**Body:**
```json
{
  "sim_id": "simulation-uuid"
}
```

Finds the first idle device and assigns the simulation to it.

**Response (200):**
```json
{
  "status": "assigned",
  "sim_id": "simulation-uuid",
  "device_id": "11e79168"
}
```

---

## Typical Workflow

1. `POST /simulations` - Create a simulation with a route
2. `POST /assignments/auto` - Auto-assign to an available device (or use manual)
3. `POST /simulations/{id}/start` - Start the simulation
4. `GET /simulations/{id}` - Poll status and current position
5. `POST /simulations/{id}/stop` - Stop when done

## Error Codes

| Status | Meaning |
|--------|---------|
| 400 | Invalid request (bad JSON, missing fields, invalid state transition) |
| 404 | Simulation or device not found |
| 500 | Internal server error |
