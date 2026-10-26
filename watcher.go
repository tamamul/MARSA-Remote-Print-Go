package main

import (
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/lxn/walk"
)

type PendingFile struct {
	Path       string
	FileName   string
	DetectedAt time.Time
}

// watchFolder pantau folder "incoming" terus-menerus di goroutine terpisah.
func watchFolder(cfg *Config, mw *walk.MainWindow) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Printf("gagal buat file watcher: %v", err)
		return
	}
	defer watcher.Close()

	if err := watcher.Add(cfg.WatchDir); err != nil {
		log.Printf("gagal pantau folder %s: %v", cfg.WatchDir, err)
		return
	}

	for {
		select {
		case event, ok := <-watcher.Events:
			if !ok {
				return
			}
			if event.Op&(fsnotify.Create|fsnotify.Write) == 0 {
				continue
			}
			if !strings.EqualFold(filepath.Ext(event.Name), ".pdf") {
				continue
			}
			go handleNewFile(event.Name, cfg, mw)

		case err, ok := <-watcher.Errors:
			if !ok {
				return
			}
			log.Printf("file watcher error: %v", err)
		}
	}
}

// handleNewFile tunggu file selesai ditulis, ambil daftar printer, lalu
// tampilkan dialog native lewat Synchronize (wajib - operasi UI walk cuma
// boleh dari thread yang sama dengan message loop).
func handleNewFile(path string, cfg *Config, mw *walk.MainWindow) {
	if !waitUntilStable(path) {
		return
	}

	item := &PendingFile{
		Path:       path,
		FileName:   filepath.Base(path),
		DetectedAt: time.Now(),
	}

	printers, err := FetchPrinters(cfg.ServerURL)

	mw.Synchronize(func() {
		if err != nil {
			walk.MsgBox(mw, "Gagal", "Gagal ambil daftar printer:\n"+err.Error(), walk.MsgBoxIconError)
			return
		}
		if len(printers) == 0 {
			walk.MsgBox(mw, "Info", "Tidak ada printer yang online saat ini.", walk.MsgBoxIconInformation)
			return
		}
		showPrintDialog(mw, cfg, item, printers)
	})
}

func waitUntilStable(path string) bool {
	var lastSize int64 = -1
	stableCount := 0

	for i := 0; i < 30; i++ { // maksimal nunggu ~15 detik
		info, err := os.Stat(path)
		if err != nil {
			time.Sleep(500 * time.Millisecond)
			continue
		}
		if info.Size() == lastSize && info.Size() > 0 {
			stableCount++
			if stableCount >= 2 {
				return true
			}
		} else {
			stableCount = 0
		}
		lastSize = info.Size()
		time.Sleep(500 * time.Millisecond)
	}
	return lastSize > 0
}
