package main

import (
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// keepFilesFor: PDF di folder "terkirim" (dan sisa file di "incoming" yang
// dibatalkan guru) dihapus otomatis setelah lewat batas ini. Dokumen sekolah
// tidak boleh menumpuk selamanya di laptop guru. Ubah angkanya kalau perlu.
const keepFilesFor = 3 * 24 * time.Hour

func cleanupOldFiles(dir string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		log.Printf("cleanup: gagal baca %s: %v", dir, err)
		return
	}
	removed := 0
	for _, e := range entries {
		if e.IsDir() || !strings.EqualFold(filepath.Ext(e.Name()), ".pdf") {
			continue
		}
		info, err := e.Info()
		if err != nil || time.Since(info.ModTime()) < keepFilesFor {
			continue
		}
		if err := os.Remove(filepath.Join(dir, e.Name())); err != nil {
			log.Printf("cleanup: gagal hapus %s: %v", e.Name(), err)
			continue
		}
		removed++
	}
	if removed > 0 {
		log.Printf("cleanup: %d file lama dihapus dari %s", removed, dir)
	}
}

// startCleanupLoop jalan di goroutine: sekali saat start, lalu tiap 6 jam.
func startCleanupLoop(cfg *Config) {
	defer recoverLog("cleanup")
	for {
		cleanupOldFiles(cfg.SentDir)
		cleanupOldFiles(cfg.WatchDir)
		time.Sleep(6 * time.Hour)
	}
}
