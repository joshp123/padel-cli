package storage

import (
	"fmt"
	"os"
	"path/filepath"
)

const (
	venuesFile   = "venues.json"
	bookingsFile = "bookings.db"
	credsFile    = "credentials.json"

	configDirEnv = "PADEL_CONFIG_DIR"
)

func ConfigDir() (string, error) {
	if dir := os.Getenv(configDirEnv); dir != "" {
		return dir, nil
	}
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, "padel"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "padel"), nil
}

func VenuesPath() (string, error) {
	dir, err := ConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, venuesFile), nil
}

func BookingsPath() (string, error) {
	dir, err := ConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, bookingsFile), nil
}

func CredentialsPath() (string, error) {
	dir, err := ConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, credsFile), nil
}

func ensureConfigDir() (string, error) {
	dir, err := ConfigDir()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("create config dir: %w", err)
	}
	return dir, nil
}
