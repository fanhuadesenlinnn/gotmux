package config

import (
	"fmt"
	"os"
	"path/filepath"
)

const DefaultLabel = "default"

func SocketPath(label, explicit string) (string, error) {
	if explicit != "" {
		return explicit, nil
	}
	if label == "" {
		label = DefaultLabel
	}
	if env := os.Getenv("GOTMUX_SOCKET"); env != "" {
		return env, nil
	}
	base := filepath.Join(os.TempDir(), fmt.Sprintf("gotmux-%d", os.Getuid()))
	if err := os.MkdirAll(base, 0o700); err != nil {
		return "", err
	}
	return filepath.Join(base, label), nil
}

func DefaultConfigFiles() []string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return nil
	}
	path := filepath.Join(home, ".tmux.conf")
	if st, err := os.Stat(path); err == nil && !st.IsDir() {
		return []string{path}
	}
	return nil
}
