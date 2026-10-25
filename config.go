package main

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type Config struct {
	ServerURL  string `json:"server_url"`  // contoh: https://chat.marsa9.com/printserver-ci4/public
	Pin        string `json:"pin"`         // PIN guru - diminta sekali saja, lalu disimpan di sini
	WatchDir   string `json:"watch_dir"`   // folder yang diawasi, dibuat otomatis kalau belum ada
	SentDir    string `json:"sent_dir"`    // folder arsip setelah berhasil diupload
	TeacherTag string `json:"teacher_tag"` // opsional, label bebas (misal nama guru) buat riwayat lokal
}

// configDir -> %APPDATA%\MarsaPrintWatcher
func configDir() (string, error) {
	appData, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(appData, "MarsaPrintWatcher")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	return dir, nil
}

func configPath() (string, error) {
	dir, err := configDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.json"), nil
}

// loadOrInitConfig baca config kalau ada; kalau belum ada, buat default
// (folder watch di dalam %APPDATA%, server_url & pin masih kosong -
// nanti diisi lewat halaman /setup).
func loadOrInitConfig() (*Config, error) {
	path, err := configPath()
	if err != nil {
		return nil, err
	}

	dir, _ := configDir()
	defaultCfg := &Config{
		WatchDir: filepath.Join(dir, "incoming"),
		SentDir:  filepath.Join(dir, "terkirim"),
	}

	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		if err := defaultCfg.Save(); err != nil {
			return nil, err
		}
		return defaultCfg, nil
	} else if err != nil {
		return nil, err
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	if cfg.WatchDir == "" {
		cfg.WatchDir = defaultCfg.WatchDir
	}
	if cfg.SentDir == "" {
		cfg.SentDir = defaultCfg.SentDir
	}
	return &cfg, nil
}

func (c *Config) Save() error {
	path, err := configPath()
	if err != nil {
		return err
	}
	_ = os.MkdirAll(c.WatchDir, 0755)
	_ = os.MkdirAll(c.SentDir, 0755)

	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

func (c *Config) IsReady() bool {
	return c.ServerURL != "" && c.Pin != ""
}
