package ui

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type Settings struct {
	SSMSPath              string `json:"ssms_path"`
	PerformanceStudioPath string `json:"performance_studio_path"`
	LastFolder            string `json:"last_folder"`
	Language              string `json:"language"` // "ET" or "EN"
}

func ExecutableDir() string {
	exe, err := os.Executable()
	if err != nil {
		return "."
	}
	return filepath.Dir(exe)
}

func LoadSettings() *Settings {
	path := filepath.Join(ExecutableDir(), "settings.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return &Settings{Language: "EN"}
	}
	var s Settings
	if err := json.Unmarshal(data, &s); err != nil {
		return &Settings{Language: "EN"}
	}
	if s.Language == "" {
		s.Language = "EN"
	}
	return &s
}

func SaveSettings(s *Settings) {
	path := filepath.Join(ExecutableDir(), "settings.json")
	data, _ := json.MarshalIndent(s, "", "  ")
	_ = os.WriteFile(path, data, 0644)
}
