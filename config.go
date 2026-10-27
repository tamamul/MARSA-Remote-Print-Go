package main

import (
	"encoding/json"
	"os"
)

// Config disimpan di file config.json satu folder dengan agent.exe
type Config struct {
	ServerURL       string `json:"server_url"`
	AgentToken      string `json:"agent_token"`
	PollIntervalSec int    `json:"poll_interval_sec"`
	HeartbeatSec    int    `json:"heartbeat_sec"`
	DashboardPort   int    `json:"dashboard_port"` // port web dashboard lokal, default 9191
	// PrintSettings diteruskan langsung ke parameter -print-settings SumatraPDF.
	// Default "fit" (shrink ke kertas default printer). Contoh buat paksa F4/Folio
	// kalau driver printernya support nama ukuran itu: "fit,paper=F4"
	// Dokumentasi opsi lengkap: https://www.sumatrapdfreader.org/docs/Command-line-arguments
	PrintSettings string `json:"print_settings"`
}

func loadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	applyDefaults(&cfg)
	return &cfg, nil
}

func applyDefaults(cfg *Config) {
	if cfg.PollIntervalSec == 0 {
		cfg.PollIntervalSec = 20
	}
	if cfg.HeartbeatSec == 0 {
		cfg.HeartbeatSec = 60
	}
	if cfg.DashboardPort == 0 {
		cfg.DashboardPort = 9191
	}
	if cfg.PrintSettings == "" {
		cfg.PrintSettings = "fit"
	}
}

// Save nulis ulang config.json, dipanggil saat user ubah setting lewat dashboard web
func (c *Config) Save(path string) error {
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}
