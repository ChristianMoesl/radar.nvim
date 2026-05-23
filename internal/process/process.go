package process

import (
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

func PIDPath() (string, error) {
	if explicit := os.Getenv("COCKPIT_PID"); explicit != "" {
		return explicit, nil
	}

	base := os.Getenv("XDG_RUNTIME_DIR")
	if base == "" {
		base = filepath.Join(os.TempDir(), "cockpit-"+os.Getenv("USER"))
	}
	return filepath.Join(base, "cockpit", "cockpit.pid"), nil
}

func WritePID() (string, error) {
	path, err := PIDPath()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return "", err
	}
	return path, os.WriteFile(path, []byte(strconv.Itoa(os.Getpid())), 0o600)
}

func ReadPID() (int, error) {
	path, err := PIDPath()
	if err != nil {
		return 0, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return 0, err
	}
	return pid, nil
}

func Stop() error {
	pids, err := DaemonPIDs()
	if err != nil {
		return err
	}
	if len(pids) == 0 {
		return nil
	}

	var lastErr error
	for _, pid := range pids {
		if err := stopPID(pid); err != nil {
			lastErr = err
		}
	}
	return lastErr
}

func DaemonPIDs() ([]int, error) {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		pid, err := ReadPID()
		if err != nil {
			return nil, err
		}
		return []int{pid}, nil
	}

	self := os.Getpid()
	pids := make([]int, 0)
	for _, entry := range entries {
		pid, err := strconv.Atoi(entry.Name())
		if err != nil || pid == self {
			continue
		}

		data, err := os.ReadFile(filepath.Join("/proc", entry.Name(), "cmdline"))
		if err != nil || len(data) == 0 {
			continue
		}

		parts := strings.Split(strings.TrimRight(string(data), "\x00"), "\x00")
		if isCockpitDaemon(parts) {
			pids = append(pids, pid)
		}
	}
	return pids, nil
}

func isCockpitDaemon(args []string) bool {
	if len(args) < 2 {
		return false
	}
	if filepath.Base(args[0]) != "cockpit" {
		return false
	}
	for _, arg := range args[1:] {
		if arg == "daemon" {
			return true
		}
	}
	return false
}

func stopPID(pid int) error {
	process, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	if err := process.Signal(syscall.SIGTERM); err != nil {
		if errors.Is(err, os.ErrProcessDone) {
			return nil
		}
		return err
	}

	for range 20 {
		if !Running(pid) {
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}

	return process.Kill()
}

func Running(pid int) bool {
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	return process.Signal(syscall.Signal(0)) == nil
}
