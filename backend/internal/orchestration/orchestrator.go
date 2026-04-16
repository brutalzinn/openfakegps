package orchestration

import (
	"fmt"
	"log"

	"github.com/openfakegps/openfakegps/backend/internal/simulation"
)

// Orchestrator coordinates simulations and devices.
type Orchestrator struct {
	registry *Registry
	engine   *simulation.Engine
}

// NewOrchestrator creates a new orchestrator.
func NewOrchestrator(registry *Registry, engine *simulation.Engine) *Orchestrator {
	return &Orchestrator{
		registry: registry,
		engine:   engine,
	}
}

// AssignSimulation manually assigns a simulation to a specific device.
func (o *Orchestrator) AssignSimulation(simID, agentID string) error {
	// Verify simulation exists.
	simInfo, err := o.engine.GetSimulation(simID)
	if err != nil {
		return fmt.Errorf("simulation lookup: %w", err)
	}
	if simInfo.DeviceID != "" {
		return fmt.Errorf("simulation %q is already assigned to device %q", simID, simInfo.DeviceID)
	}

	// Verify device exists and is idle.
	dev, ok := o.registry.Get(agentID)
	if !ok {
		return fmt.Errorf("device %q not found", agentID)
	}
	if dev.Status != StatusIdle {
		return fmt.Errorf("device %q is not idle (status: %s)", agentID, dev.Status)
	}

	// Perform assignment.
	if err := o.engine.SetDeviceID(simID, agentID); err != nil {
		return err
	}
	o.registry.UpdateStatus(agentID, StatusAssigned)
	o.registry.SetAssignedSim(agentID, simID)

	log.Printf("assigned simulation %s to device %s", simID, agentID)
	return nil
}

// AutoAssign finds an idle device and assigns the simulation to it.
func (o *Orchestrator) AutoAssign(simID string) (string, error) {
	// Verify simulation exists.
	simInfo, err := o.engine.GetSimulation(simID)
	if err != nil {
		return "", fmt.Errorf("simulation lookup: %w", err)
	}
	if simInfo.DeviceID != "" {
		return "", fmt.Errorf("simulation %q is already assigned to device %q", simID, simInfo.DeviceID)
	}

	// Find an idle device.
	dev := o.registry.FindIdle()
	if dev == nil {
		return "", fmt.Errorf("no idle devices available")
	}

	// Perform assignment.
	if err := o.engine.SetDeviceID(simID, dev.AgentID); err != nil {
		return "", err
	}
	o.registry.UpdateStatus(dev.AgentID, StatusAssigned)
	o.registry.SetAssignedSim(dev.AgentID, simID)

	log.Printf("auto-assigned simulation %s to device %s", simID, dev.AgentID)
	return dev.AgentID, nil
}

// UnassignSimulation removes the device assignment from a simulation.
func (o *Orchestrator) UnassignSimulation(simID string) error {
	simInfo, err := o.engine.GetSimulation(simID)
	if err != nil {
		return fmt.Errorf("simulation lookup: %w", err)
	}
	if simInfo.DeviceID == "" {
		return fmt.Errorf("simulation %q is not assigned to any device", simID)
	}

	agentID := simInfo.DeviceID

	if err := o.engine.ClearDeviceID(simID); err != nil {
		return err
	}
	o.registry.UpdateStatus(agentID, StatusIdle)
	o.registry.SetAssignedSim(agentID, "")

	log.Printf("unassigned simulation %s from device %s", simID, agentID)
	return nil
}

// HandleDisconnect is called when an agent disconnects. It stops any running
// simulation and marks the device as disconnected.
func (o *Orchestrator) HandleDisconnect(agentID string) {
	dev, ok := o.registry.Get(agentID)
	if !ok {
		return
	}

	// If the device had an assigned simulation, stop it.
	if dev.AssignedSim != "" {
		if err := o.engine.StopSimulation(dev.AssignedSim); err != nil {
			log.Printf("error stopping simulation %s on disconnect: %v", dev.AssignedSim, err)
		}
		_ = o.engine.ClearDeviceID(dev.AssignedSim)
	}

	o.registry.UpdateStatus(agentID, StatusDisconnected)
	o.registry.SetAssignedSim(agentID, "")
	log.Printf("device %s disconnected", agentID)
}

// HandleReconnect restores a previously known device to idle status.
func (o *Orchestrator) HandleReconnect(agentID string) {
	dev, ok := o.registry.Get(agentID)
	if !ok {
		return
	}

	if dev.Status == StatusDisconnected {
		o.registry.UpdateStatus(agentID, StatusIdle)
		o.registry.UpdateLastSeen(agentID)
		log.Printf("device %s reconnected", agentID)
	}
}
