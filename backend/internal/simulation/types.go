package simulation

import (
	"context"
	"sync"
	"time"
)

// State represents the lifecycle state of a simulation.
type State int

const (
	StateCreated State = iota
	StateRunning
	StatePaused
	StateStopped
)

// String returns a human-readable state name.
func (s State) String() string {
	switch s {
	case StateCreated:
		return "created"
	case StateRunning:
		return "running"
	case StatePaused:
		return "paused"
	case StateStopped:
		return "stopped"
	default:
		return "unknown"
	}
}

// Waypoint is a single GPS coordinate in a route.
type Waypoint struct {
	Lat, Lon float64
}

// Config holds the parameters for creating a simulation.
type Config struct {
	ID             string
	Route          []Waypoint
	SpeedMps       float64       // target speed in m/s, default 13.8 (~50 km/h)
	UpdateInterval time.Duration // default 1s
	NoiseMeters    float64       // GPS noise radius, default 2.0
	Stops          []StopConfig  // optional planned stops
}

// StopConfig defines a planned stop at a waypoint.
type StopConfig struct {
	WaypointIndex int
	Duration      time.Duration
}

// Position represents a GPS position at a point in time.
type Position struct {
	Lat       float64
	Lon       float64
	Speed     float64 // m/s
	Bearing   float64 // degrees
	Accuracy  float64 // meters
	Altitude  float64
	Timestamp time.Time
}

// Simulation holds the runtime state for a single simulation.
type Simulation struct {
	Config   Config
	State    State
	CurrentPos Position
	DeviceID string // assigned device agent ID

	// Internal fields for tracking progress along the route.
	waypointIdx int
	segmentFrac float64
	mu          sync.RWMutex
	cancel      context.CancelFunc
	done        chan struct{}
	prevBearing float64
}

// StatusInfo is a snapshot of a simulation for external consumption.
type StatusInfo struct {
	ID         string   `json:"id"`
	State      string   `json:"state"`
	DeviceID   string   `json:"device_id"`
	CurrentPos Position `json:"current_pos"`
	Progress   float64  `json:"progress"` // 0.0 to 1.0
}
