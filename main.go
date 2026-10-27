package main

import (
	_ "embed"
	"log"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/lxn/walk"
)

//go:embed icon.ico
var iconBytes []byte

// loadIcon tulis icon.ico ke temp file sekali, lalu load lewat walk.
// (walk butuh path file, bukan bytes langsung)
func loadIcon() (*walk.Icon, error) {
	tmpFile := filepath.Join(os.TempDir(), "marsa-print-watcher.ico")
	if _, err := os.Stat(tmpFile); os.IsNotExist(err) {
		if err := os.WriteFile(tmpFile, iconBytes, 0644); err != nil {
			return nil, err
		}
	}
	return walk.NewIconFromFile(tmpFile)
}

func main() {
	cfg, err := loadOrInitConfig()
	if err != nil {
		log.Fatalf("gagal load config: %v", err)
	}

	mw, err := walk.NewMainWindow()
	if err != nil {
		log.Fatal(err)
	}
	mw.SetVisible(false) // jendela utama cuma buat message loop, tidak kelihatan

	icon, err := loadIcon()
	if err != nil {
		log.Fatalf("gagal load icon: %v", err)
	}

	ni, err := walk.NewNotifyIcon(mw)
	if err != nil {
		log.Fatal(err)
	}
	defer ni.Dispose()
	_ = ni.SetIcon(icon)
	_ = ni.SetToolTip("MARSA Print")
	_ = ni.SetVisible(true)

	menu := ni.ContextMenu()

	actFolder := walk.NewAction()
	_ = actFolder.SetText("Buka Folder Cetak")
	actFolder.Triggered().Attach(func() {
		_ = exec.Command("explorer", cfg.WatchDir).Start()
	})
	_ = menu.Actions().Add(actFolder)

	actSetup := walk.NewAction()
	_ = actSetup.SetText("Setting")
	actSetup.Triggered().Attach(func() {
		showSetupDialog(mw, cfg)
	})
	_ = menu.Actions().Add(actSetup)

	_ = menu.Actions().Add(walk.NewSeparatorAction())

	actExit := walk.NewAction()
	_ = actExit.SetText("Keluar")
	actExit.Triggered().Attach(func() {
		walk.App().Exit(0)
	})
	_ = menu.Actions().Add(actExit)

	// Klik ganti ikon = buka Setting (perilaku umum tray app)
	ni.MouseDown().Attach(func(x, y int, button walk.MouseButton) {
		if button == walk.LeftButton {
			showSetupDialog(mw, cfg)
		}
	})

	if !cfg.IsReady() {
		showSetupDialog(mw, cfg)
	}

	go watchFolder(cfg, mw)

	mw.Run()
}
