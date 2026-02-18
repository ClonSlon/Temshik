package appdata

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const appDirName = ".temchik"

type Settings struct {
	Host string `json:"host"`
	Port int    `json:"port"`
}

var DefaultSettings = Settings{
	Host: "localhost",
	Port: 8000,
}

func Dir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, appDirName), nil
}

func EnsureDir() (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	return dir, nil
}

func SettingsPath() (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "settings.json"), nil
}

func LogPath() (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "log.log"), nil
}

func PassPath() (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "pass"), nil
}

func PIDPath() (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "pid"), nil
}

func DBPath() (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "dev.db"), nil
}

func DBURL() (string, error) {
	dbPath, err := DBPath()
	if err != nil {
		return "", err
	}
	return "file:" + dbPath, nil
}

func ReadSettings() (Settings, error) {
	settingsPath, err := SettingsPath()
	if err != nil {
		return Settings{}, err
	}

	data, err := os.ReadFile(settingsPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			_ = WriteSettings(DefaultSettings)
			return DefaultSettings, nil
		}
		return Settings{}, err
	}

	var s Settings
	if err := json.Unmarshal(data, &s); err != nil {
		return DefaultSettings, nil
	}

	if s.Host == "" {
		s.Host = DefaultSettings.Host
	}
	if s.Port == 0 {
		s.Port = DefaultSettings.Port
	}

	return s, nil
}

func WriteSettings(settings Settings) error {
	if _, err := EnsureDir(); err != nil {
		return err
	}
	settingsPath, err := SettingsPath()
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(settingsPath, data, 0o644)
}

func ReadPassword() (string, error) {
	passPath, err := PassPath()
	if err != nil {
		return "", err
	}
	data, err := os.ReadFile(passPath)
	if err != nil {
		return "", err
	}
	pass := strings.TrimSpace(string(data))
	if pass == "" {
		return "", fmt.Errorf("empty password file: %s", passPath)
	}
	return pass, nil
}

func WritePassword(password string) error {
	if _, err := EnsureDir(); err != nil {
		return err
	}
	passPath, err := PassPath()
	if err != nil {
		return err
	}
	return os.WriteFile(passPath, []byte(password), 0o600)
}

func ReadPID() (int, error) {
	pidPath, err := PIDPath()
	if err != nil {
		return 0, err
	}
	data, err := os.ReadFile(pidPath)
	if err != nil {
		return 0, err
	}
	pidStr := strings.TrimSpace(string(data))
	if pidStr == "" {
		return 0, os.ErrNotExist
	}
	pid, err := strconv.Atoi(pidStr)
	if err != nil || pid <= 0 {
		return 0, fmt.Errorf("invalid pid file: %s", pidPath)
	}
	return pid, nil
}

func WritePID(pid int) error {
	if _, err := EnsureDir(); err != nil {
		return err
	}
	pidPath, err := PIDPath()
	if err != nil {
		return err
	}
	return os.WriteFile(pidPath, []byte(strconv.Itoa(pid)), 0o644)
}

func ClearPID() error {
	if _, err := EnsureDir(); err != nil {
		return err
	}
	pidPath, err := PIDPath()
	if err != nil {
		return err
	}
	return os.WriteFile(pidPath, nil, 0o644)
}
