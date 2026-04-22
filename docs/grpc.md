# gRPC Communication Protocol

Proto definition: `proto/fakegps/v1/fakegps.proto`

Default port: `50051`

---

## Service: AgentService

### RPC: Connect

```protobuf
rpc Connect(stream AgentMessage) returns (stream ServerMessage);
```

Bidirectional streaming RPC. The Android agent opens a persistent connection and exchanges messages with the backend.

---

## Connection Lifecycle

### 1. Registration (Agent -> Server)

First message **must** be a `RegisterRequest`:

```
AgentMessage {
  register: RegisterRequest {
    agent_id: "08291JEC212687"
    device_name: "sunfish"
    device_model: "Pixel 4a"
    capabilities: ["mock_location", "gps_simulation"]
  }
}
```

> **Note:** The `agent_id` is the device's ADB serial number (same value from `adb devices`). It is passed to the app via intent extra: `--es device_serial <SERIAL>`.

Server responds with `RegisterResponse`:

```
ServerMessage {
  register_response: RegisterResponse {
    accepted: true
    message: "registered"
  }
}
```

### 2. Heartbeat (Agent -> Server)

Agent sends periodic heartbeats (recommended: every 10 seconds):

```
AgentMessage {
  heartbeat: Heartbeat {
    timestamp_ms: 1704067200000
  }
}
```

### 3. Status Updates (Agent -> Server)

Agent reports state changes:

```
AgentMessage {
  status_update: StatusUpdate {
    agent_id: "08291JEC212687"
    status: DEVICE_STATUS_SIMULATING
    message: "GPS injection active"
  }
}
```

### 4. Location Updates (Server -> Agent)

When a simulation is running and assigned to this device:

```
ServerMessage {
  location_update: LocationUpdate {
    simulation_id: "sim-uuid"
    latitude: -23.5505
    longitude: -46.6333
    speed: 13.8         // m/s
    bearing: 45.2       // degrees
    accuracy: 3.5       // meters
    altitude: 760.0
    timestamp_ms: 1704067200000
  }
}
```

Updates are streamed at the simulation's configured interval (default: 1 second).

### 5. Simulation Commands (Server -> Agent)

```
ServerMessage {
  simulation_command: SimulationCommand {
    action: SIMULATION_ACTION_START
    simulation_id: "sim-uuid"
  }
}
```

Actions: `START`, `STOP`, `PAUSE`, `RESUME`.

### 6. Disconnection

When the stream closes (EOF or error), the backend:
- Stops any running simulation assigned to the device
- Marks the device as `DISCONNECTED`
- Clears the simulation assignment

On reconnect with the same `agent_id`, the device is restored to `IDLE`.

---

## Enums

### SimulationAction
| Value | Number |
|-------|--------|
| SIMULATION_ACTION_UNSPECIFIED | 0 |
| SIMULATION_ACTION_START | 1 |
| SIMULATION_ACTION_STOP | 2 |
| SIMULATION_ACTION_PAUSE | 3 |
| SIMULATION_ACTION_RESUME | 4 |

### DeviceStatus
| Value | Number |
|-------|--------|
| DEVICE_STATUS_UNSPECIFIED | 0 |
| DEVICE_STATUS_IDLE | 1 |
| DEVICE_STATUS_ASSIGNED | 2 |
| DEVICE_STATUS_SIMULATING | 3 |
| DEVICE_STATUS_DISCONNECTED | 4 |

---

## Reconnection Strategy

The Android agent should implement exponential backoff:
- Initial delay: 1 second
- Multiplier: 2x
- Maximum delay: 30 seconds
- On successful reconnect, send `RegisterRequest` with the same `agent_id`
