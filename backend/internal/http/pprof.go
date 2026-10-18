package http

import (
	"context"
	"errors"
	"feedsystem_video_go/internal/config"
	"feedsystem_video_go/internal/pkg/logger"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/http/pprof"
	"time"
)

type PprofServer struct {
	server          *http.Server
	shutdownTimeout time.Duration
}

func NewPprofMux() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/debug/pprof/", pprof.Index)
	mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
	mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	mux.HandleFunc("/debug/pprof/trace", pprof.Trace)

	return mux
}

func NewPprofServer(cfg *config.PprofConfig) (*PprofServer, error) {
	if cfg == nil || !cfg.Enabled || cfg.Addr == "" {
		return nil, nil
	}
	ln, err := net.Listen("tcp", cfg.Addr)
	if err != nil {
		return nil, fmt.Errorf("failed to start pprof server on %s: %w", cfg.Addr, err)
	}
	pprofServer := &PprofServer{
		server: &http.Server{
			Handler:     NewPprofMux(),
			ReadTimeout: 5 * time.Second,
		},
		shutdownTimeout: 3 * time.Second,
	}
	go func() {
		logger.Info("Pprof listening", slog.String("addr", cfg.Addr))
		if err := pprofServer.server.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("Pprof server failed", slog.Any("error", err))
		}
	}()
	return pprofServer, nil
}

func (s *PprofServer) Shutdown() {
	if s == nil {
		return
	}
	shutdownCtx, cancel := context.WithTimeout(context.Background(), s.shutdownTimeout)
	defer cancel()
	if err := s.server.Shutdown(shutdownCtx); err != nil {
		logger.Warn("Pprof shutdown failed", slog.Any("error", err))
		return
	}
	logger.Info("Pprof shutdown success")
}
