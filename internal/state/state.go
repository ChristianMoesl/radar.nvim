package state

import (
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"sync"

	"radar.nvim/internal/protocol"
)

type Store struct {
	mu       sync.RWMutex
	items    []protocol.Item
	services []protocol.ServiceStatus
	path     string
	logger   *slog.Logger
}

func Path() (string, error) {
	if explicit := os.Getenv("RADAR_STATE"); explicit != "" {
		return explicit, nil
	}

	base := os.Getenv("XDG_STATE_HOME")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		base = filepath.Join(home, ".local", "state")
	}

	return filepath.Join(base, "radar", "items.json"), nil
}

func NewStore(logger *slog.Logger) (*Store, error) {
	path, err := Path()
	if err != nil {
		return nil, err
	}

	store := &Store{
		items:  []protocol.Item{},
		path:   path,
		logger: logger,
	}
	if err := store.Load(); err != nil {
		logger.Warn("could not load state", "path", path, "error", err)
	}
	return store, nil
}

func (s *Store) Load() error {
	data, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			s.logger.Info("state file does not exist yet", "path", s.path)
			return nil
		}
		return err
	}

	var items []protocol.Item
	if err := json.Unmarshal(data, &items); err != nil {
		return err
	}

	s.mu.Lock()
	s.items = items
	s.mu.Unlock()

	s.logger.Info("state loaded", "path", s.path, "items", len(items))
	return nil
}

func (s *Store) SetItems(items []protocol.Item) {
	s.mu.Lock()
	s.items = make([]protocol.Item, len(items))
	copy(s.items, items)
	s.mu.Unlock()

	if err := s.Save(); err != nil {
		s.logger.Warn("could not save state", "path", s.path, "error", err)
	}
}

func (s *Store) Save() error {
	s.mu.RLock()
	items := make([]protocol.Item, len(s.items))
	copy(items, s.items)
	s.mu.RUnlock()

	if err := os.MkdirAll(filepath.Dir(s.path), 0o700); err != nil {
		return err
	}

	data, err := json.MarshalIndent(items, "", "  ")
	if err != nil {
		return err
	}

	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	if err := os.Rename(tmp, s.path); err != nil {
		return err
	}

	s.logger.Debug("state saved", "path", s.path, "items", len(items))
	return nil
}

func (s *Store) Items() []protocol.Item {
	s.mu.RLock()
	defer s.mu.RUnlock()

	items := make([]protocol.Item, len(s.items))
	copy(items, s.items)
	return items
}

func (s *Store) SetServices(services []protocol.ServiceStatus) {
	s.mu.Lock()
	s.services = make([]protocol.ServiceStatus, len(services))
	copy(s.services, services)
	s.mu.Unlock()
}

func (s *Store) Services() []protocol.ServiceStatus {
	s.mu.RLock()
	defer s.mu.RUnlock()

	services := make([]protocol.ServiceStatus, len(s.services))
	copy(services, s.services)
	return services
}

func (s *Store) Summary() protocol.Summary {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var summary protocol.Summary
	for _, item := range s.items {
		switch item.Attention {
		case "immediate":
			summary.Immediate++
		case "attention":
			summary.Attention++
		case "in_progress":
			summary.InProgress++
		case "done":
			summary.Done++
		}
	}
	return summary
}
