package orchestration

import (
	"sync"
	"time"

	"github.com/openfakegps/openfakegps/backend/internal/simulation"
)

// DeviceStatus represents the current state of a connected device.
type DeviceStatus string

const (
	StatusIdle         DeviceStatus = "idle"
	StatusAssigned     DeviceStatus = "assigned"
	StatusSimulating   DeviceStatus = "simulating"
	StatusDisconnected DeviceStatus = "disconnected"
)

// ServerStream is the interface for sending messages to a connected agent.
type ServerStream interface {
	SendLocationUpdate(simID string, pos simulation.Position) error
	SendSimulationCommand(action string, simID string) error
}

// DeviceInfo holds metadata and runtime state for a connected device.
type DeviceInfo struct {
	AgentID      string       `json:"agent_id"`
	DeviceName   string       `json:"device_name"`
	Model        string       `json:"model"`
	Capabilities []string     `json:"capabilities"`
	Status       DeviceStatus `json:"status"`
	AssignedSim  string       `json:"assigned_sim"`
	LastSeen     time.Time    `json:"last_seen"`
	Stream       ServerStream `json:"-"`
}

// Registry is a concurrent-safe store of connected devices.
type Registry struct {
	mu      sync.RWMutex
	devices map[string]*DeviceInfo
}

// NewRegistry creates an empty device registry.
func NewRegistry() *Registry {
	return &Registry{
		devices: make(map[string]*DeviceInfo),
	}
}

// Register adds or updates a device in the registry.
func (r *Registry) Register(info DeviceInfo) {
	r.mu.Lock()
	defer r.mu.Unlock()

	info.LastSeen = time.Now()
	if info.Status == "" {
		info.Status = StatusIdle
	}
	r.devices[info.AgentID] = &info
}

// Unregister removes a device from the registry.
func (r *Registry) Unregister(agentID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.devices, agentID)
}

// Get returns a copy of a device's info.
func (r *Registry) Get(agentID string) (*DeviceInfo, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	d, ok := r.devices[agentID]
	if !ok {
		return nil, false
	}
	cp := *d
	return &cp, true
}

// List returns copies of all registered devices.
func (r *Registry) List() []DeviceInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()

	list := make([]DeviceInfo, 0, len(r.devices))
	for _, d := range r.devices {
		list = append(list, *d)
	}
	return list
}

// UpdateStatus sets the status of a device.
func (r *Registry) UpdateStatus(agentID string, status DeviceStatus) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if d, ok := r.devices[agentID]; ok {
		d.Status = status
	}
}

// UpdateLastSeen refreshes the last-seen timestamp for a device.
func (r *Registry) UpdateLastSeen(agentID string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if d, ok := r.devices[agentID]; ok {
		d.LastSeen = time.Now()
	}
}

// FindIdle returns the first idle device, or nil if none are available.
func (r *Registry) FindIdle() *DeviceInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, d := range r.devices {
		if d.Status == StatusIdle {
			cp := *d
			return &cp
		}
	}
	return nil
}

// SetStream assigns a gRPC stream to a device.
func (r *Registry) SetStream(agentID string, stream ServerStream) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if d, ok := r.devices[agentID]; ok {
		d.Stream = stream
	}
}

// GetStream returns the stream associated with a device.
func (r *Registry) GetStream(agentID string) ServerStream {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if d, ok := r.devices[agentID]; ok {
		return d.Stream
	}
	return nil
}

// SetAssignedSim sets the assigned simulation for a device.
func (r *Registry) SetAssignedSim(agentID, simID string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if d, ok := r.devices[agentID]; ok {
		d.AssignedSim = simID
	}
}
