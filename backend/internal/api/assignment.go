package api

import (
	"encoding/json"
	"net/http"
)

type assignRequest struct {
	SimID    string `json:"sim_id"`
	DeviceID string `json:"device_id"`
}

type autoAssignRequest struct {
	SimID string `json:"sim_id"`
}

func (s *Server) assignSimulation(w http.ResponseWriter, r *http.Request) {
	var req assignRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}

	if req.SimID == "" || req.DeviceID == "" {
		writeError(w, http.StatusBadRequest, "sim_id and device_id are required")
		return
	}

	// Stop any active position hold on this device before assigning a simulation.
	stopPositionHold(req.DeviceID)

	if err := s.orchestrator.AssignSimulation(req.SimID, req.DeviceID); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"status":    "assigned",
		"sim_id":    req.SimID,
		"device_id": req.DeviceID,
	})
}

func (s *Server) unassignSimulation(w http.ResponseWriter, r *http.Request) {
	simID := r.PathValue("sim_id")
	if simID == "" {
		writeError(w, http.StatusBadRequest, "sim_id is required")
		return
	}

	if err := s.orchestrator.UnassignSimulation(simID); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"status": "unassigned",
		"sim_id": simID,
	})
}

func (s *Server) autoAssign(w http.ResponseWriter, r *http.Request) {
	var req autoAssignRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}

	if req.SimID == "" {
		writeError(w, http.StatusBadRequest, "sim_id is required")
		return
	}

	deviceID, err := s.orchestrator.AutoAssign(req.SimID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"status":    "assigned",
		"sim_id":    req.SimID,
		"device_id": deviceID,
	})
}
