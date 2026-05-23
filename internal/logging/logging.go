package logging

import (
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/lmittmann/tint"
)

func Path() (string, error) {
	if explicit := os.Getenv("COCKPIT_LOG"); explicit != "" {
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

	return filepath.Join(base, "cockpit", "cockpit.log"), nil
}

func New() (*slog.Logger, *os.File, string, error) {
	path, err := Path()
	if err != nil {
		return nil, nil, "", err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, nil, "", err
	}

	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return nil, nil, "", err
	}

	return NewWithWriter(file), file, path, nil
}

func NewWithWriter(w io.Writer) *slog.Logger {
	level := slog.LevelDebug
	addSource := isDevelopment()

	handler := tint.NewHandler(w, &tint.Options{
		Level:      level,
		AddSource:  addSource,
		TimeFormat: time.DateTime,
		NoColor:    !useColor(),
	})

	return slog.New(handler)
}

func isDevelopment() bool {
	env := strings.ToLower(os.Getenv("COCKPIT_ENV"))
	return env == "" || env == "dev" || env == "development"
}

func useColor() bool {
	value := strings.ToLower(os.Getenv("COCKPIT_LOG_COLOR"))
	return value != "0" && value != "false" && value != "no"
}
