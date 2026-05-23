package server

import (
	"bufio"
	"encoding/json"
	"log/slog"
	"net"
	"os"

	"radar.nvim/internal/protocol"
	"radar.nvim/internal/socket"
	"radar.nvim/internal/state"
)

type Server struct {
	store   *state.Store
	logger  *slog.Logger
	refresh func()
}

func New(store *state.Store, logger *slog.Logger, refresh func()) *Server {
	return &Server{store: store, logger: logger, refresh: refresh}
}

func (s *Server) ListenAndServe(path string) error {
	if err := socket.EnsureDir(path); err != nil {
		return err
	}
	_ = os.Remove(path)

	listener, err := net.Listen("unix", path)
	if err != nil {
		return err
	}
	defer listener.Close()

	s.logger.Info("server listening", "socket", path)

	for {
		conn, err := listener.Accept()
		if err != nil {
			s.logger.Error("accept failed", "error", err)
			return err
		}
		s.logger.Debug("client connected")
		go s.handle(conn)
	}
}

func (s *Server) handle(conn net.Conn) {
	defer conn.Close()
	defer s.logger.Debug("client disconnected")

	scanner := bufio.NewScanner(conn)
	encoder := json.NewEncoder(conn)

	for scanner.Scan() {
		var req protocol.Request
		if err := json.Unmarshal(scanner.Bytes(), &req); err != nil {
			s.logger.Warn("invalid request", "error", err)
			_ = encoder.Encode(protocol.Response{OK: false, Error: err.Error()})
			continue
		}

		s.logger.Debug("request received", "method", req.Method)
		switch req.Method {
		case "summary":
			summary := s.store.Summary()
			_ = encoder.Encode(protocol.Response{OK: true, Summary: &summary})
		case "items":
			summary := s.store.Summary()
			_ = encoder.Encode(protocol.Response{OK: true, Summary: &summary, Items: s.store.Items()})
		case "refresh":
			if s.refresh != nil {
				s.refresh()
			}
			summary := s.store.Summary()
			_ = encoder.Encode(protocol.Response{OK: true, Summary: &summary, Items: s.store.Items()})
		default:
			s.logger.Warn("unknown method", "method", req.Method)
			_ = encoder.Encode(protocol.Response{OK: false, Error: "unknown method: " + req.Method})
		}
	}

	if err := scanner.Err(); err != nil {
		s.logger.Warn("client read failed", "error", err)
	}
}
