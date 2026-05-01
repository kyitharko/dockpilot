package api

import (
	"encoding/json"
	"net/http"
	"sort"
	"strconv"
	"strings"

	"dockpilot/internal/engine"
	"dockpilot/internal/services"
)

// handleHealth pings the Docker daemon and returns its reachability.
//
// GET /health
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	if err := s.eng.Health(r.Context()); err != nil {
		errUnavailable(w, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// handleListServices lists all built-in service definitions from the registry.
//
// GET /v1/services
func (s *Server) handleListServices(w http.ResponseWriter, r *http.Request) {
	all := services.All()

	type svcResponse struct {
		Name    string   `json:"name"`
		Image   string   `json:"image"`
		Ports   []string `json:"ports"`
		Volumes []string `json:"volumes"`
		Env     []string `json:"env"`
	}

	resp := make([]svcResponse, 0, len(all))
	for _, def := range all {
		resp = append(resp, svcResponse{
			Name:    def.Name,
			Image:   def.Image,
			Ports:   def.Ports,
			Volumes: def.Volumes,
			Env:     def.Env,
		})
	}
	sort.Slice(resp, func(i, j int) bool { return resp[i].Name < resp[j].Name })
	writeJSON(w, http.StatusOK, resp)
}

// handleDeploy deploys a service container.
// The {service} path param is the service name; optional JSON body overrides image/ports/env/volumes.
//
// POST /v1/services/{service}/deploy
func (s *Server) handleDeploy(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("service")
	if name == "" {
		errBadRequest(w, "service name is required")
		return
	}

	var req engine.DeployRequest
	req.Name = name

	// Body is optional — omitting it deploys a built-in service with default config.
	if r.ContentLength != 0 {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			errBadRequest(w, "invalid JSON body: "+err.Error())
			return
		}
		req.Name = name // path param always wins
	}

	result, err := s.eng.Deploy(r.Context(), req)
	if err != nil {
		if strings.Contains(err.Error(), "already exists") {
			errConflict(w, err.Error())
			return
		}
		errInternal(w, err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, result)
}

// handleRemove stops and removes a service container.
// Optional query param ?volumes=vol1,vol2 removes the listed named volumes.
//
// DELETE /v1/services/{service}
func (s *Server) handleRemove(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("service")

	var volumes []string
	if v := r.URL.Query().Get("volumes"); v != "" {
		for _, vol := range strings.Split(v, ",") {
			if vol = strings.TrimSpace(vol); vol != "" {
				volumes = append(volumes, vol)
			}
		}
	}

	if err := s.eng.Remove(r.Context(), name, volumes); err != nil {
		if strings.Contains(err.Error(), "not found") {
			errNotFound(w, err.Error())
			return
		}
		errInternal(w, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "removed", "name": name})
}

// handleStatus returns the runtime state of a service container.
//
// GET /v1/services/{service}/status
func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("service")

	status, err := s.eng.Status(r.Context(), name)
	if err != nil {
		errInternal(w, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, status)
}

// handleLogs returns the last N lines of a service container's logs.
// Query param ?tail=N controls the number of lines (default 100).
//
// GET /v1/services/{service}/logs
func (s *Server) handleLogs(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("service")

	tail := 100
	if t := r.URL.Query().Get("tail"); t != "" {
		n, err := strconv.Atoi(t)
		if err != nil || n <= 0 {
			errBadRequest(w, "tail must be a positive integer")
			return
		}
		tail = n
	}

	lines, err := s.eng.Logs(r.Context(), name, tail)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			errNotFound(w, err.Error())
			return
		}
		errInternal(w, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"container": "dockpilot-" + name,
		"logs":      lines,
	})
}
