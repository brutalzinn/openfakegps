package api

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/openfakegps/openfakegps/backend/internal/simulation"
	"github.com/openfakegps/openfakegps/backend/pkg/geo"
)

type createSimulationRequest struct {
	Route []struct {
		Lat float64 `json:"lat"`
		Lon float64 `json:"lon"`
	} `json:"route"`
	Polyline       string   `json:"polyline,omitempty"`   // Google encoded polyline (mutually exclusive with route)
	SpeedMps       *float64 `json:"speed_mps,omitempty"`  // speed in meters per second
	SpeedKmh       *float64 `json:"speed_kmh,omitempty"`  // speed in km/h (converted to m/s internally)
	UpdateInterval string   `json:"update_interval"`      // e.g. "1s"
	NoiseMeters    float64  `json:"noise_meters"`
	Stops          []struct {
		WaypointIndex int    `json:"waypoint_index"`
		Duration      string `json:"duration"`
	} `json:"stops"`
}

func (s *Server) createSimulation(w http.ResponseWriter, r *http.Request) {
	var req createSimulationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}

	if len(req.Route) > 0 && req.Polyline != "" {
		writeError(w, http.StatusBadRequest, "provide either route or polyline, not both")
		return
	}
	if len(req.Route) == 0 && req.Polyline == "" {
		writeError(w, http.StatusBadRequest, "provide either route or polyline")
		return
	}

	if req.SpeedMps != nil && req.SpeedKmh != nil {
		writeError(w, http.StatusBadRequest, "provide either speed_mps or speed_kmh, not both")
		return
	}

	var speedMps float64
	switch {
	case req.SpeedKmh != nil:
		speedMps = *req.SpeedKmh / 3.6
	case req.SpeedMps != nil:
		speedMps = *req.SpeedMps
	}

	var waypoints []simulation.Waypoint
	if req.Polyline != "" {
		points := geo.DecodePolyline(req.Polyline)
		if len(points) < 2 {
			writeError(w, http.StatusBadRequest, "polyline must decode to at least 2 points")
			return
		}
		waypoints = make([]simulation.Waypoint, len(points))
		for i, p := range points {
			waypoints[i] = simulation.Waypoint{Lat: p[0], Lon: p[1]}
		}
	} else {
		waypoints = make([]simulation.Waypoint, len(req.Route))
		for i, wp := range req.Route {
			waypoints[i] = simulation.Waypoint{Lat: wp.Lat, Lon: wp.Lon}
		}
	}

	interval := time.Second
	if req.UpdateInterval != "" {
		d, err := time.ParseDuration(req.UpdateInterval)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid update_interval: "+err.Error())
			return
		}
		interval = d
	}

	var stops []simulation.StopConfig
	for _, sc := range req.Stops {
		d, err := time.ParseDuration(sc.Duration)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid stop duration: "+err.Error())
			return
		}
		stops = append(stops, simulation.StopConfig{
			WaypointIndex: sc.WaypointIndex,
			Duration:      d,
		})
	}

	cfg := simulation.Config{
		Route:          waypoints,
		SpeedMps:       speedMps,
		UpdateInterval: interval,
		NoiseMeters:    req.NoiseMeters,
		Stops:          stops,
	}

	sim, err := s.engine.CreateSimulation(cfg)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, map[string]string{"id": sim.Config.ID})
}

func (s *Server) listSimulations(w http.ResponseWriter, _ *http.Request) {
	list := s.engine.ListSimulations()
	writeJSON(w, http.StatusOK, list)
}

func (s *Server) getSimulation(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	info, err := s.engine.GetSimulation(id)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, info)
}

func (s *Server) startSimulation(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := s.engine.StartSimulation(id); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "started"})
}

func (s *Server) pauseSimulation(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := s.engine.PauseSimulation(id); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "paused"})
}

func (s *Server) resumeSimulation(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := s.engine.ResumeSimulation(id); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "resumed"})
}

func (s *Server) stopSimulation(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := s.engine.StopSimulation(id); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "stopped"})
}
