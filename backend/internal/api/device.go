package api

import (
	"net/http"
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
