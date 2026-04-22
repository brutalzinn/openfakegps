# openfakegps

A cross-platform GPS simulation system for automated ride-hailing testing. Built entirely with Claude as a PoC, but fully usable for automations with apps that need GPS location tracking (Uber, delivery apps, etc).

Extensible to other platforms if needed. Currently supports Android devices using the Mock Location Provider API.

Uses gRPC for real-time communication between multiple devices. The Go backend controls devices using goroutines for fast, concurrent multi-device simulation.

## Device Identification

Devices are identified by their **ADB serial number** — the same value shown by `adb devices`. This makes it easy to integrate with ADB-based automation workflows.

```bash
# ADB serial matches the device_id in the API
$ adb devices
08291JEC212687    device
11e79168          device

# Use the same serial to assign simulations
curl -X POST http://localhost:3000/api/v1/assignments \
  -d '{"sim_id": "...", "device_id": "08291JEC212687"}'
```

The serial is passed to the Android agent via intent extra when launching:
```bash
adb -s <SERIAL> shell am start -n com.openfakegps.agent/.MainActivity --es device_serial <SERIAL>
```

## How to Use

### Backend

```bash
# Using Go
cd backend && go run ./cmd/server

# Or with Docker
docker compose up -d
```

### Android Agent

Build and install with Android Studio (compileSdk 35, minSdk 26) or via CLI:

```bash
cd android
JAVA_HOME=/path/to/jdk17 ./gradlew assembleDebug
```

Install on all connected devices:
```bash
for SERIAL in $(adb devices | tail -n +2 | awk '{print $1}'); do
  adb -s $SERIAL install -r android/app/build/outputs/apk/debug/app-debug.apk
  adb -s $SERIAL shell pm grant com.openfakegps.agent android.permission.ACCESS_FINE_LOCATION
  adb -s $SERIAL shell pm grant com.openfakegps.agent android.permission.ACCESS_COARSE_LOCATION
  adb -s $SERIAL shell pm grant com.openfakegps.agent android.permission.READ_PHONE_STATE
  adb -s $SERIAL shell am start -n com.openfakegps.agent/.MainActivity --es device_serial $SERIAL
done
```

You also need to set the app as the mock location provider manually in Developer Options on each device.

## Documentation

1. **[Agent Tool Guide](docs/agent-tool-guide.md)** — How to use this project as a tool for AI agents and automation scripts (includes ride-hailing workflow with Appium)
2. **[REST API](docs/api.md)** — Full API reference
3. **[gRPC Protocol](docs/grpc.md)** — Device communication protocol
4. **Swagger UI** — Available at `http://localhost:3000/swagger` when the backend is running

Feel free to contribute with an iOS version.

