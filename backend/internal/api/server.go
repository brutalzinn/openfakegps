package api

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/openfakegps/openfakegps/backend/internal/orchestration"
	"github.com/openfakegps/openfakegps/backend/internal/simulation"
)

// Server provides the REST API.
type Server struct {
	orchestrator *orchestration.Orchestrator
	engine       *simulation.Engine
	registry     *orchestration.Registry
}

// NewServer creates a new API server.
func NewServer(orch *orchestration.Orchestrator, engine *simulation.Engine, reg *orchestration.Registry) *Server {
	return &Server{
		orchestrator: orch,
		engine:       engine,
		registry:     reg,
	}
}

// Routes returns an http.Handler with all API routes registered.
func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()

	// Simulation routes.
	mux.HandleFunc("POST /api/v1/simulations", s.createSimulation)
	mux.HandleFunc("GET /api/v1/simulations", s.listSimulations)
	mux.HandleFunc("GET /api/v1/simulations/{id}", s.getSimulation)
	mux.HandleFunc("POST /api/v1/simulations/{id}/start", s.startSimulation)
	mux.HandleFunc("POST /api/v1/simulations/{id}/pause", s.pauseSimulation)
	mux.HandleFunc("POST /api/v1/simulations/{id}/resume", s.resumeSimulation)
	mux.HandleFunc("POST /api/v1/simulations/{id}/stop", s.stopSimulation)
	mux.HandleFunc("POST /api/v1/simulations/{id}/position", s.setPosition)

	// Device routes.
	mux.HandleFunc("GET /api/v1/devices", s.listDevices)
	mux.HandleFunc("GET /api/v1/devices/{id}", s.getDevice)
	mux.HandleFunc("POST /api/v1/devices/position", s.setDevicePosition)
	mux.HandleFunc("POST /api/v1/devices/position/stop", s.stopDevicePosition)

	// Assignment routes.
	mux.HandleFunc("POST /api/v1/assignments", s.assignSimulation)
	mux.HandleFunc("POST /api/v1/assignments/auto", s.autoAssign)
	mux.HandleFunc("DELETE /api/v1/assignments/{sim_id}", s.unassignSimulation)

	// Swagger UI (served outside JSON middleware).
	RegisterSwagger(mux)

	return chain(mux, corsMiddleware, jsonMiddleware, loggingMiddleware)
}

// chain applies middleware in order (last applied wraps outermost).
func chain(h http.Handler, mws ...func(http.Handler) http.Handler) http.Handler {
	for i := len(mws) - 1; i >= 0; i-- {
		h = mws[i](h)
	}
	return h
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func jsonMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") {
			w.Header().Set("Content-Type", "application/json")
		}
		next.ServeHTTP(w, r)
	})
}

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("%s %s %s", r.Method, r.URL.Path, time.Since(start))
	})
}

// writeJSON encodes v as JSON and writes it to w.
func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("error encoding JSON response: %v", err)
	}
}

// writeError writes a JSON error response.
func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
