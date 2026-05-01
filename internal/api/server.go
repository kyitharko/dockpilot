package api

import (
	"context"
	"net/http"

	"dockpilot/internal/engine"
)

// Server is the HTTP API server. Both its address and the underlying engine are
// set at construction time; the server itself is stateless across requests.
type Server struct {
	eng  *engine.Engine
	addr string
}

// New returns a Server bound to addr (e.g. "127.0.0.1:8088").
func New(eng *engine.Engine, addr string) *Server {
	return &Server{eng: eng, addr: addr}
}

// Serve starts the HTTP server and blocks until ctx is cancelled, then performs
// a graceful 30-second shutdown — draining in-flight requests before returning.
func (s *Server) Serve(ctx context.Context) error {
	srv := &http.Server{
		Addr:    s.addr,
		Handler: s.buildMux(),
	}

	// Start shutdown watcher in background.
	go func() {
		<-ctx.Done()
		shutCtx, cancel := context.WithTimeout(context.Background(), 30*1e9) // 30s
		defer cancel()
		srv.Shutdown(shutCtx) //nolint:errcheck
	}()

	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return err
	}
	return nil
}
