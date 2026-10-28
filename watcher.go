package main

import (
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/lxn/walk"
	"github.com/lxn/win"
)

type PendingFile struct {
	Path       string
	FileName   string
	DetectedAt time.Time
}

// watchFolder pantau folder "incoming" terus-menerus di goroutine terpisah.
func watchFolder(cfg *Config, mw *walk.MainWindow) {
	defer recoverLog("watchFolder")
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
			log.Printf("PDF terdeteksi: %s", event.Name)
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
	defer recoverLog("handleNewFile")
	if !waitUntilStable(path) {
		return
	}

	item := &PendingFile{
		Path:       path,
		FileName:   filepath.Base(path),
		DetectedAt: time.Now(),
	}

	log.Printf("file stabil: %s, ambil daftar printer...", item.FileName)
	printers, err := FetchPrinters(cfg.ServerURL)
	if err != nil {
		log.Printf("FetchPrinters GAGAL: %v", err)
	} else {
		log.Printf("FetchPrinters OK: %d printer online", len(printers))
	}

	mw.Synchronize(func() {
		defer recoverLog("dialog cetak")
		if err != nil {
			walk.MsgBox(nil, "Gagal", "Gagal ambil daftar printer:\n"+err.Error(), walk.MsgBoxIconError|walk.MsgBoxStyle(win.MB_TOPMOST))
			return
		}
		if len(printers) == 0 {
			walk.MsgBox(nil, "Info", "Tidak ada printer yang online saat ini.", walk.MsgBoxIconInformation|walk.MsgBoxStyle(win.MB_TOPMOST))
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
