package simulation

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
)

// LocationCallback is invoked each tick with the new position for a simulation.
type LocationCallback func(simID, deviceID string, pos Position)

// Engine manages the lifecycle of all simulations.
type Engine struct {
	mu          sync.RWMutex
	simulations map[string]*Simulation
	callback    LocationCallback
}

// NewEngine creates a new simulation engine with the given location callback.
func NewEngine(cb LocationCallback) *Engine {
	return &Engine{
		simulations: make(map[string]*Simulation),
		callback:    cb,
	}
}

// CreateSimulation validates the config and stores a new simulation.
func (e *Engine) CreateSimulation(cfg Config) (*Simulation, error) {
	if len(cfg.Route) < 2 {
		return nil, fmt.Errorf("route must have at least 2 waypoints, got %d", len(cfg.Route))
	}

	if cfg.ID == "" {
		cfg.ID = uuid.New().String()
	}
	if cfg.SpeedMps <= 0 {
		cfg.SpeedMps = 13.8 // ~50 km/h
	}
	if cfg.UpdateInterval <= 0 {
		cfg.UpdateInterval = time.Second
	}
	if cfg.NoiseMeters <= 0 {
		cfg.NoiseMeters = 2.0
	}
	if cfg.AccelMps2 <= 0 {
		cfg.AccelMps2 = 2.5
	}
	if cfg.DecelMps2 <= 0 {
		cfg.DecelMps2 = 3.0
	}
	if cfg.BearingSmooth <= 0 {
		cfg.BearingSmooth = 0.3
	}

	sim := &Simulation{
		Config: cfg,
		State:  StateCreated,
		CurrentPos: Position{
			Lat:       cfg.Route[0].Lat,
			Lon:       cfg.Route[0].Lon,
			Timestamp: time.Now(),
		},
		done:           make(chan struct{}),
		completedStops: make(map[int]bool),
	}

	e.mu.Lock()
	e.simulations[cfg.ID] = sim
	e.mu.Unlock()

	return sim, nil
}

// StartSimulation begins ticking the simulation identified by id.
func (e *Engine) StartSimulation(id string) error {
	e.mu.RLock()
	sim, ok := e.simulations[id]
	e.mu.RUnlock()
	if !ok {
		return fmt.Errorf("simulation %q not found", id)
	}

	sim.mu.Lock()
	if sim.State == StateRunning {
		sim.mu.Unlock()
		return fmt.Errorf("simulation %q is already running", id)
	}
	if sim.State == StateStopped {
		sim.mu.Unlock()
		return fmt.Errorf("simulation %q is stopped and cannot be restarted", id)
	}

	ctx, cancel := context.WithCancel(context.Background())
	sim.cancel = cancel
	sim.State = StateRunning
	sim.done = make(chan struct{})
	sim.mu.Unlock()

	go e.runLoop(ctx, sim)
	return nil
}

// PauseSimulation pauses a running simulation.
// The run loop keeps running and continues to send the current position
// so the device maintains the mock location instead of reverting to real GPS.
func (e *Engine) PauseSimulation(id string) error {
	e.mu.RLock()
	sim, ok := e.simulations[id]
	e.mu.RUnlock()
	if !ok {
		return fmt.Errorf("simulation %q not found", id)
	}

	sim.mu.Lock()
	defer sim.mu.Unlock()

	if sim.State != StateRunning {
		return fmt.Errorf("simulation %q is not running (state: %s)", id, sim.State)
	}

	sim.State = StatePaused
	return nil
}

// ResumeSimulation resumes a paused simulation.
func (e *Engine) ResumeSimulation(id string) error {
	e.mu.RLock()
	sim, ok := e.simulations[id]
	e.mu.RUnlock()
	if !ok {
		return fmt.Errorf("simulation %q not found", id)
	}

	sim.mu.Lock()
	defer sim.mu.Unlock()

	if sim.State != StatePaused {
		return fmt.Errorf("simulation %q is not paused (state: %s)", id, sim.State)
	}

	sim.State = StateRunning
	return nil
}

// StopSimulation permanently stops a simulation.
func (e *Engine) StopSimulation(id string) error {
	e.mu.RLock()
	sim, ok := e.simulations[id]
	e.mu.RUnlock()
	if !ok {
		return fmt.Errorf("simulation %q not found", id)
	}

	sim.mu.Lock()
	prevState := sim.State
	sim.State = StateStopped
	if sim.cancel != nil {
		sim.cancel()
	}
	sim.mu.Unlock()

	// Wait for the loop to finish if it was running, paused, or completed (loop stays alive).
	if prevState == StateRunning || prevState == StatePaused || prevState == StateCompleted {
		<-sim.done
	}
	return nil
}

// GetSimulation returns a status snapshot for the given simulation.
func (e *Engine) GetSimulation(id string) (*StatusInfo, error) {
	e.mu.RLock()
	sim, ok := e.simulations[id]
	e.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("simulation %q not found", id)
	}

	sim.mu.RLock()
	defer sim.mu.RUnlock()

	return &StatusInfo{
		ID:         sim.Config.ID,
		State:      sim.State.String(),
		DeviceID:   sim.DeviceID,
		CurrentPos: sim.CurrentPos,
		Progress:   progress(sim),
	}, nil
}

// ListSimulations returns status snapshots for all simulations.
func (e *Engine) ListSimulations() []StatusInfo {
	e.mu.RLock()
	defer e.mu.RUnlock()

	list := make([]StatusInfo, 0, len(e.simulations))
	for _, sim := range e.simulations {
		sim.mu.RLock()
		list = append(list, StatusInfo{
			ID:         sim.Config.ID,
			State:      sim.State.String(),
			DeviceID:   sim.DeviceID,
			CurrentPos: sim.CurrentPos,
			Progress:   progress(sim),
		})
		sim.mu.RUnlock()
	}
	return list
}

// SetDeviceID assigns a device to a simulation.
func (e *Engine) SetDeviceID(simID, deviceID string) error {
	e.mu.RLock()
	sim, ok := e.simulations[simID]
	e.mu.RUnlock()
	if !ok {
		return fmt.Errorf("simulation %q not found", simID)
	}

	sim.mu.Lock()
	sim.DeviceID = deviceID
	sim.mu.Unlock()
	return nil
}

// ClearDeviceID removes the device assignment from a simulation.
func (e *Engine) ClearDeviceID(simID string) error {
	e.mu.RLock()
	sim, ok := e.simulations[simID]
	e.mu.RUnlock()
	if !ok {
		return fmt.Errorf("simulation %q not found", simID)
	}

	sim.mu.Lock()
	sim.DeviceID = ""
	sim.mu.Unlock()
	return nil
}

// SetPosition manually sets the current position of a simulation and pushes it to the device.
// Works only when the simulation is Running or Paused.
func (e *Engine) SetPosition(id string, lat, lon, speed, bearing float64) (*Position, error) {
	e.mu.RLock()
	sim, ok := e.simulations[id]
	e.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("simulation %q not found", id)
	}

	sim.mu.Lock()
	if sim.State != StateRunning && sim.State != StatePaused {
		state := sim.State
		sim.mu.Unlock()
		return nil, fmt.Errorf("simulation %q is %s; must be running or paused", id, state)
	}

	pos := Position{
		Lat:       lat,
		Lon:       lon,
		Speed:     speed,
		Bearing:   bearing,
		Accuracy:  0,
		Altitude:  sim.CurrentPos.Altitude,
		Timestamp: time.Now(),
	}
	sim.CurrentPos = pos

	simID := sim.Config.ID
	deviceID := sim.DeviceID
	sim.mu.Unlock()

	if e.callback != nil && deviceID != "" {
		e.callback(simID, deviceID, pos)
	}

	return &pos, nil
}

// runLoop is the main tick loop for a simulation, running in its own goroutine.
func (e *Engine) runLoop(ctx context.Context, sim *Simulation) {
	defer func() {
		close(sim.done)
	}()

	// Send the starting position immediately so the device updates
	// before the first tick, avoiding a brief teleport from the old location.
	sim.mu.Lock()
	if sim.State == StateRunning {
		startPos := sim.CurrentPos
		simID := sim.Config.ID
		deviceID := sim.DeviceID
		sim.mu.Unlock()
		if e.callback != nil && deviceID != "" {
			e.callback(simID, deviceID, startPos)
		}
	} else {
		sim.mu.Unlock()
	}

	ticker := time.NewTicker(sim.Config.UpdateInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			sim.mu.Lock()
			state := sim.State
			if state == StateStopped {
				sim.mu.Unlock()
				return
			}

			simID := sim.Config.ID
			deviceID := sim.DeviceID

			if state == StatePaused || state == StateCompleted {
				// Keep sending current position to maintain mock location on device.
				pos := sim.CurrentPos
				sim.mu.Unlock()

				if e.callback != nil && deviceID != "" {
					e.callback(simID, deviceID, pos)
				}
				continue
			}

			pos := advancePosition(sim, sim.Config.UpdateInterval)
			pos = addNoise(pos, sim.Config.NoiseMeters)
			sim.CurrentPos = pos

			// When we reach the end of the route, switch to completed state
			// to keep sending the final position and prevent teleporting.
			if sim.waypointIdx >= len(sim.Config.Route)-1 {
				sim.State = StateCompleted
				sim.CurrentPos.Speed = 0
				sim.mu.Unlock()

				if e.callback != nil && deviceID != "" {
					e.callback(simID, deviceID, pos)
				}
				continue
			}

			sim.mu.Unlock()

			if e.callback != nil && deviceID != "" {
				e.callback(simID, deviceID, pos)
			}
		}
	}
}
