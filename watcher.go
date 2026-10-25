package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"
)

// watchFolder pantau folder "incoming" terus-menerus. Dipanggil sebagai
// goroutine terpisah, jalan selama aplikasi hidup.
func watchFolder(cfg *Config, queue *Queue, port int) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		logErr("gagal buat file watcher: %v", err)
		return
	}
	defer watcher.Close()

	if err := watcher.Add(cfg.WatchDir); err != nil {
		logErr("gagal pantau folder %s: %v", cfg.WatchDir, err)
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
			go handleNewFile(event.Name, queue, port)

		case err, ok := <-watcher.Errors:
			if !ok {
				return
			}
			logErr("file watcher error: %v", err)
		}
	}
}

// handleNewFile tunggu sampai ukuran file berhenti berubah (proses "Save As"
// dari printer selesai nulis), baru dianggap siap diupload. PDF besar bisa
// butuh beberapa detik untuk selesai ditulis ke disk.
func handleNewFile(path string, queue *Queue, port int) {
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
				break
			}
		} else {
			stableCount = 0
		}
		lastSize = info.Size()
		time.Sleep(500 * time.Millisecond)
	}

	item := queue.Add(path, filepath.Base(path))
	url := fmt.Sprintf("http://127.0.0.1:%d/print?id=%s", port, item.ID)
	openBrowser(url)
}
