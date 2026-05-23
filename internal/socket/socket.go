package socket

import (
	"errors"
	"os"
	"path/filepath"
)

func Path() (string, error) {
	if explicit := os.Getenv("COCKPIT_SOCKET"); explicit != "" {
		return explicit, nil
	}

	base := os.Getenv("XDG_RUNTIME_DIR")
	if base == "" {
		base = filepath.Join(os.TempDir(), "cockpit-"+os.Getenv("USER"))
	}
	if base == "" {
		return "", errors.New("could not determine runtime directory")
	}
	return filepath.Join(base, "cockpit", "cockpit.sock"), nil
}

func EnsureDir(path string) error {
	return os.MkdirAll(filepath.Dir(path), 0o700)
}
