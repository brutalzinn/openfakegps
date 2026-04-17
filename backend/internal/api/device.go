package api

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/openfakegps/openfakegps/backend/internal/simulation"
)

func (s *Server) listDevices(w http.ResponseWriter, _ *http.Request) {
	devices := s.registry.List()
	writeJSON(w, http.StatusOK, devices)
}

func (s *Server) getDevice(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	dev, ok := s.registry.Get(id)
	if !ok {
		writeError(w, http.StatusNotFound, "device not found")
		return
	}
	writeJSON(w, http.StatusOK, dev)
}

type setDevicePositionRequest struct {
	DeviceID string  `json:"device_id"`
	Lat      float64 `json:"lat"`
	Lon      float64 `json:"lon"`
	Speed    float64 `json:"speed"`
	Bearing  float64 `json:"bearing"`
}

// positionHold tracks a background goroutine that keeps sending a fixed position to a device.
type positionHold struct {
	cancel context.CancelFunc
}

var (
	positionHolds   = make(map[string]*positionHold) // keyed by device_id
	positionHoldsMu sync.Mutex
)

func (s *Server) setDevicePosition(w http.ResponseWriter, r *http.Request) {
	var req setDevicePositionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}

	if req.DeviceID == "" {
		writeError(w, http.StatusBadRequest, "device_id is required")
		return
	}

	stream := s.registry.GetStream(req.DeviceID)
	if stream == nil {
		writeError(w, http.StatusNotFound, "device not connected")
		return
	}

	// Cancel any existing position hold for this device.
	positionHoldsMu.Lock()
	if prev, ok := positionHolds[req.DeviceID]; ok {
		prev.cancel()
	}

	ctx, cancel := context.WithCancel(context.Background())
	positionHolds[req.DeviceID] = &positionHold{cancel: cancel}
	positionHoldsMu.Unlock()

	// Start background loop to keep sending the position.
	go func() {
		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				stream := s.registry.GetStream(req.DeviceID)
				if stream == nil {
					stopPositionHold(req.DeviceID)
					return
				}
				pos := simulation.Position{
					Lat:       req.Lat,
					Lon:       req.Lon,
					Speed:     req.Speed,
					Bearing:   req.Bearing,
					Accuracy:  1.0,
					Timestamp: time.Now(),
				}
				if err := stream.SendLocationUpdate("", pos); err != nil {
					log.Printf("position hold: failed to send to %s: %v", req.DeviceID, err)
					return
				}
			}
		}
	}()

	// Send the first update immediately.
	pos := simulation.Position{
		Lat:       req.Lat,
		Lon:       req.Lon,
		Speed:     req.Speed,
		Bearing:   req.Bearing,
		Accuracy:  1.0,
		Timestamp: time.Now(),
	}

	if err := stream.SendLocationUpdate("", pos); err != nil {
		cancel()
		writeError(w, http.StatusInternalServerError, "failed to send position: "+err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status": "position_set",
		"lat":    req.Lat,
		"lon":    req.Lon,
	})
}

// stopPositionHold cancels any active position hold for a device.
func stopPositionHold(deviceID string) {
	positionHoldsMu.Lock()
	if hold, ok := positionHolds[deviceID]; ok {
		hold.cancel()
		delete(positionHolds, deviceID)
	}
	positionHoldsMu.Unlock()
}

// stopDevicePosition stops the position hold for a device.
func (s *Server) stopDevicePosition(w http.ResponseWriter, r *http.Request) {
	var req struct {
		DeviceID string `json:"device_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}

	if req.DeviceID == "" {
		writeError(w, http.StatusBadRequest, "device_id is required")
		return
	}

	stopPositionHold(req.DeviceID)

	writeJSON(w, http.StatusOK, map[string]string{"status": "position_hold_stopped"})
}
