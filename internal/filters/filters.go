package filters

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"radar.nvim/internal/protocol"
)

type Config struct {
	MuteRepos         []string `json:"mute_repos,omitempty"`
	DeprioritizeRepos []string `json:"deprioritize_repos,omitempty"`
	MuteUsers         []string `json:"mute_users,omitempty"`
	DeprioritizeUsers []string `json:"deprioritize_users,omitempty"`
}

func Path() (string, error) {
	if explicit := os.Getenv("RADAR_FILTERS"); explicit != "" {
		return explicit, nil
	}

	base := os.Getenv("XDG_CONFIG_HOME")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		base = filepath.Join(home, ".config")
	}

	return filepath.Join(base, "radar", "filters.json"), nil
}

func Load() (Config, error) {
	path, err := Path()
	if err != nil {
		return Config{}, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return Config{}, nil
		}
		return Config{}, err
	}
	if len(strings.TrimSpace(string(data))) == 0 {
		return Config{}, nil
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func EnsureFile() (string, error) {
	path, err := Path()
	if err != nil {
		return "", err
	}
	if _, err := os.Stat(path); err == nil {
		return path, nil
	} else if !os.IsNotExist(err) {
		return "", err
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return "", err
	}
	data := []byte("{\n  \"mute_repos\": [],\n  \"deprioritize_repos\": [],\n  \"mute_users\": [],\n  \"deprioritize_users\": []\n}\n")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return "", err
	}
	return path, nil
}

func Apply(items []protocol.Item, cfg Config) []protocol.Item {
	muteRepos := set(cfg.MuteRepos)
	lowRepos := set(cfg.DeprioritizeRepos)
	muteUsers := set(cfg.MuteUsers)
	lowUsers := set(cfg.DeprioritizeUsers)

	filtered := make([]protocol.Item, 0, len(items))
	for _, item := range items {
		if matchesItem(item, muteRepos, muteUsers) {
			continue
		}

		if matchesItem(item, lowRepos, lowUsers) {
			item.Attention = "low_priority"
			if item.Reason != "" && !strings.HasPrefix(item.Reason, "low priority") {
				item.Reason = "low priority: " + item.Reason
			} else if item.Reason == "" {
				item.Reason = "low priority"
			}
		}
		filtered = append(filtered, item)
	}
	return filtered
}

func Summary(items []protocol.Item) protocol.Summary {
	var summary protocol.Summary
	for _, item := range items {
		switch item.Attention {
		case "immediate":
			summary.Immediate++
		case "attention":
			summary.Attention++
		case "in_progress":
			summary.InProgress++
		case "done":
			summary.Done++
		case "low_priority":
			summary.LowPriority++
		}
	}
	return summary
}

func matchesItem(item protocol.Item, repos map[string]bool, users map[string]bool) bool {
	if len(repos) > 0 {
		if repos[normalize(item.Repo)] {
			return true
		}
		for _, entity := range item.Entities {
			if repos[normalize(entity.Repo)] {
				return true
			}
		}
	}

	if len(users) > 0 {
		if users[normalize(metadata(item.Metadata, "user", "author", "login"))] {
			return true
		}
		for _, entity := range item.Entities {
			if users[normalize(metadata(entity.Metadata, "user", "author", "login"))] {
				return true
			}
		}
	}

	return false
}

func metadata(values map[string]string, keys ...string) string {
	for _, key := range keys {
		if values[key] != "" {
			return values[key]
		}
	}
	return ""
}

func set(values []string) map[string]bool {
	result := make(map[string]bool, len(values))
	for _, value := range values {
		value = normalize(value)
		if value != "" {
			result[value] = true
		}
	}
	return result
}

func normalize(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}
