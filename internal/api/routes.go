package api

import "net/http"

// buildMux wires all API routes to their handlers and applies middleware.
func (s *Server) buildMux() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /health", s.handleHealth)
	mux.HandleFunc("GET /v1/services", s.handleListServices)
	mux.HandleFunc("POST /v1/services/{service}/deploy", s.handleDeploy)
	mux.HandleFunc("DELETE /v1/services/{service}", s.handleRemove)
	mux.HandleFunc("GET /v1/services/{service}/status", s.handleStatus)
	mux.HandleFunc("GET /v1/services/{service}/logs", s.handleLogs)

	return chain(requestLogger, recoverer)(mux)
}
