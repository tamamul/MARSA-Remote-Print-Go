package main

import (
	_ "embed"
	"fmt"
	"log"
	"net/http"
	"os/exec"
	"runtime"

	"github.com/getlantern/systray"
)

//go:embed icon.ico
var iconBytes []byte

const dashboardPort = 9292

type App struct {
	cfg   *Config
	queue *Queue
}

func logErr(format string, args ...interface{}) {
	log.Printf(format, args...)
}

func openBrowser(url string) {
	if runtime.GOOS != "windows" {
		return
	}
	_ = exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
}

func main() {
	cfg, err := loadOrInitConfig()
	if err != nil {
		log.Fatalf("gagal load config: %v", err)
	}

	app := &App{cfg: cfg, queue: NewQueue()}

	mux := http.NewServeMux()
	mux.HandleFunc("/", app.handleHome)
	mux.HandleFunc("/setup", app.handleSetup)
	mux.HandleFunc("/print", app.handlePrintPage)
	mux.HandleFunc("/submit", app.handleSubmit)

	go func() {
		addr := fmt.Sprintf("127.0.0.1:%d", dashboardPort)
		if err := http.ListenAndServe(addr, mux); err != nil {
			log.Printf("web server gagal start: %v", err)
		}
	}()

	go watchFolder(cfg, app.queue, dashboardPort)

	// Kalau setup belum diisi, langsung buka halaman setup sekali di awal
	if !cfg.IsReady() {
		openBrowser(fmt.Sprintf("http://127.0.0.1:%d/setup", dashboardPort))
	}

	systray.Run(func() {
		systray.SetIcon(iconBytes)
		systray.SetTitle("MARSA Print")
		systray.SetTooltip("MARSA Print - klik kanan untuk menu")

		mOpen := systray.AddMenuItem("Buka MARSA Print", "Lihat status")
		mFolder := systray.AddMenuItem("Buka Folder Cetak", "Tempat simpan PDF dari Word/Excel")
		mSetup := systray.AddMenuItem("Setting", "Ubah URL server / PIN")
		systray.AddSeparator()
		mQuit := systray.AddMenuItem("Keluar", "Tutup MARSA Print")

		go func() {
			for {
				select {
				case <-mOpen.ClickedCh:
					openBrowser(fmt.Sprintf("http://127.0.0.1:%d/", dashboardPort))
				case <-mFolder.ClickedCh:
					_ = exec.Command("explorer", cfg.WatchDir).Start()
				case <-mSetup.ClickedCh:
					openBrowser(fmt.Sprintf("http://127.0.0.1:%d/setup", dashboardPort))
				case <-mQuit.ClickedCh:
					systray.Quit()
					return
				}
			}
		}()
	}, func() {})
}
