package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/lxn/walk"
	. "github.com/lxn/walk/declarative"
	"github.com/lxn/win"
)

const baseFontFamily = "Segoe UI"

var (
	colorOK   = walk.RGB(15, 157, 88)
	colorErr  = walk.RGB(209, 52, 56)
	colorWait = walk.RGB(202, 80, 16)
)

var pinRe = regexp.MustCompile(`^[0-9]{5}$`)

// bringToFront: dialog dari proses background (watcher) gampang "tenggelam"
// di belakang Word karena Windows melarang app non-aktif merebut fokus.
// Dibuat TOPMOST supaya pasti kelihatan walau tidak bisa merebut fokus.
func bringToFront(dlg *walk.Dialog) {
	h := dlg.Handle()
	win.SetWindowPos(h, win.HWND_TOPMOST, 0, 0, 0, 0, win.SWP_NOMOVE|win.SWP_NOSIZE|win.SWP_SHOWWINDOW)
	win.SetForegroundWindow(h)
}

// runOnTop: dialog dibuat dulu (Create), jalankan hook onReady, lalu Run.
// Owner sengaja nil - lihat catatan di main.go soal panic walk.
func runOnTop(d Dialog, dlg **walk.Dialog, onReady ...func()) {
	if err := d.Create(nil); err != nil {
		log.Printf("gagal membuat dialog: %v", err)
		walk.MsgBox(nil, "Error", "Gagal membuat jendela: "+err.Error(), walk.MsgBoxIconError|walk.MsgBoxStyle(win.MB_TOPMOST))
		return
	}
	for _, f := range onReady {
		f()
	}
	(*dlg).Starting().Attach(func() { bringToFront(*dlg) })
	(*dlg).Run()
}

func titleFont() Font { return Font{Family: baseFontFamily, PointSize: 15, Bold: true} }

// ---------------------------------------------------------------- Setup

func showSetupDialog(owner walk.Form, cfg *Config) {
	defer owner.SetVisible(false) // pengaman: jendela utama harus tetap tersembunyi
	var dlg *walk.Dialog
	var serverEdit, pinEdit *walk.LineEdit
	var saveBtn, cancelBtn *walk.PushButton

	runOnTop(Dialog{
		AssignTo:      &dlg,
		Title:         "Pengaturan MARSA Print",
		MinSize:       Size{Width: 540, Height: 330},
		Font:          Font{Family: baseFontFamily, PointSize: 11},
		DefaultButton: &saveBtn,
		CancelButton:  &cancelBtn,
		Layout:        VBox{Margins: Margins{Left: 24, Top: 20, Right: 24, Bottom: 20}, Spacing: 10},
		Children: []Widget{
			Label{Text: "Pengaturan MARSA Print", Font: titleFont()},
			Label{Text: "Diisi sekali saja. PIN disimpan di laptop ini."},
			Label{Text: "URL Server:"},
			LineEdit{AssignTo: &serverEdit, Text: cfg.ServerURL, CueBanner: "https://..."},
			Label{Text: "PIN Guru (5 digit angka):"},
			LineEdit{AssignTo: &pinEdit, Text: cfg.Pin, PasswordMode: true, MaxLength: 5},
			VSpacer{},
			Composite{
				Layout: HBox{MarginsZero: true, Spacing: 10},
				Children: []Widget{
					HSpacer{},
					PushButton{
						AssignTo: &saveBtn,
						Text:     "Simpan",
						MinSize:  Size{Width: 120, Height: 40},
						OnClicked: func() {
							url := strings.TrimRight(strings.TrimSpace(serverEdit.Text()), "/")
							pin := strings.TrimSpace(pinEdit.Text())
							if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
								walk.MsgBox(dlg, "Periksa isian", "URL Server harus diawali http:// atau https://", walk.MsgBoxIconWarning)
								return
							}
							if !pinRe.MatchString(pin) {
								walk.MsgBox(dlg, "Periksa isian", "PIN harus 5 digit angka.", walk.MsgBoxIconWarning)
								return
							}
							cfg.ServerURL = url
							cfg.Pin = pin
							_ = cfg.Save()
							dlg.Accept()
						},
					},
					PushButton{AssignTo: &cancelBtn, Text: "Batal", MinSize: Size{Width: 110, Height: 40}, OnClicked: func() { dlg.Cancel() }},
				},
			},
		},
	}, &dlg)
}

// ---------------------------------------------------------------- Cetak

// printUI menampung semua widget dialog cetak supaya bisa diubah dari
// goroutine (lewat owner.Synchronize) saat upload & polling status berjalan.
type printUI struct {
	owner        *walk.MainWindow
	dlg          *walk.Dialog
	formBox      *walk.Composite
	statusBox    *walk.Composite
	statusTitle  *walk.Label
	statusDetail *walk.Label
	errLbl       *walk.Label
	bar          *walk.ProgressBar
	printBtn     *walk.PushButton
	cancelBtn    *walk.PushButton
	closed       atomic.Bool // true setelah dialog ditutup - hentikan semua update UI
}

func (ui *printUI) setStatus(title, detail string, c walk.Color, busy bool) {
	_ = ui.statusTitle.SetText(title)
	ui.statusTitle.SetTextColor(c)
	_ = ui.statusDetail.SetText(detail)
	ui.bar.SetVisible(busy)
}

// ui menjalankan fungsi f di UI thread, dengan pengaman panic dan cek dialog
// sudah ditutup atau belum.
func (ui *printUI) onUI(where string, f func()) {
	ui.owner.Synchronize(func() {
		defer recoverLog(where)
		if ui.closed.Load() {
			return
		}
		f()
	})
}

func showPrintDialog(owner *walk.MainWindow, cfg *Config, item *PendingFile, printers []TeacherPrinter) {
	defer owner.SetVisible(false) // pengaman: jendela utama harus tetap tersembunyi
	ui := &printUI{owner: owner}
	var combo, copiesCombo *walk.ComboBox

	names := make([]string, len(printers))
	for i, p := range printers {
		names[i] = fmt.Sprintf("%s  (%s)", p.Nama, p.Lokasi)
	}
	copyItems := []string{"1", "2", "3", "4", "5", "6", "7", "8", "9", "10"}

	runOnTop(Dialog{
		AssignTo:      &ui.dlg,
		Title:         "Cetak Dokumen",
		MinSize:       Size{Width: 580, Height: 440},
		Font:          Font{Family: baseFontFamily, PointSize: 11},
		DefaultButton: &ui.printBtn,
		CancelButton:  &ui.cancelBtn,
		Layout:        VBox{Margins: Margins{Left: 24, Top: 20, Right: 24, Bottom: 20}, Spacing: 10},
		Children: []Widget{
			Label{Text: "Cetak Dokumen", Font: titleFont()},
			Label{Text: item.FileName, Font: Font{Family: baseFontFamily, PointSize: 11, Bold: true}},

			// --- bagian form (disembunyikan setelah job terkirim) ---
			Composite{
				AssignTo: &ui.formBox,
				Layout:   VBox{MarginsZero: true, Spacing: 8},
				Children: []Widget{
					Label{Text: "Pilih printer:"},
					ComboBox{AssignTo: &combo, Model: names, CurrentIndex: 0},
					Label{Text: "Jumlah salinan (maks 10):"},
					ComboBox{AssignTo: &copiesCombo, Model: copyItems, CurrentIndex: 0},
				},
			},
			Label{AssignTo: &ui.errLbl, Text: " "},

			// --- bagian status (muncul setelah klik Cetak) ---
			Composite{
				AssignTo: &ui.statusBox,
				Visible:  false,
				Layout:   VBox{MarginsZero: true, Spacing: 8},
				Children: []Widget{
					Label{AssignTo: &ui.statusTitle, Text: "Menunggu di antrean", Font: titleFont()},
					Label{AssignTo: &ui.statusDetail, Text: "Agent di komputer printer akan mengambil job ini sebentar lagi."},
					ProgressBar{AssignTo: &ui.bar, MarqueeMode: true},
				},
			},

			VSpacer{},
			Composite{
				Layout: HBox{MarginsZero: true, Spacing: 10},
				Children: []Widget{
					HSpacer{},
					PushButton{
						AssignTo: &ui.printBtn,
						Text:     "Cetak Sekarang",
						MinSize:  Size{Width: 170, Height: 42},
						OnClicked: func() {
							idx := combo.CurrentIndex()
							if idx < 0 {
								return
							}
							printerID, err := strconv.Atoi(printers[idx].ID)
							if err != nil {
								_ = ui.errLbl.SetText("ID printer tidak valid.")
								ui.errLbl.SetTextColor(colorErr)
								return
							}
							copies := copiesCombo.CurrentIndex() + 1

							_ = ui.errLbl.SetText(" ")
							ui.printBtn.SetEnabled(false)
							ui.cancelBtn.SetEnabled(false)
							ui.formBox.SetVisible(false)
							ui.statusBox.SetVisible(true)
							ui.setStatus("Mengirim ke server...", "Mohon tunggu, dokumen sedang diunggah.", colorWait, true)

							go ui.submit(cfg, item, printerID, copies)
						},
					},
					PushButton{
						AssignTo:  &ui.cancelBtn,
						Text:      "Batal",
						MinSize:   Size{Width: 120, Height: 42},
						OnClicked: func() { ui.dlg.Cancel() },
					},
				},
			},
		},
	}, &ui.dlg, func() {
		ui.dlg.Disposing().Attach(func() { ui.closed.Store(true) })
	})
}

// submit: upload di goroutine (jendela tidak freeze), lalu lanjut polling.
func (ui *printUI) submit(cfg *Config, item *PendingFile, printerID, copies int) {
	defer recoverLog("submit")
	log.Printf("submit job: file=%s printer_id=%d copies=%d", item.FileName, printerID, copies)
	result, err := SubmitJob(cfg.ServerURL, cfg.Pin, item.Path, printerID, copies)

	if err != nil {
		log.Printf("submit job GAGAL: %v", err)
		ui.onUI("submit gagal", func() {
			ui.statusBox.SetVisible(false)
			ui.formBox.SetVisible(true)
			_ = ui.errLbl.SetText("Gagal: " + truncate(err.Error(), 160))
			ui.errLbl.SetTextColor(colorErr)
			ui.printBtn.SetEnabled(true)
			ui.cancelBtn.SetEnabled(true)
		})
		return
	}
	log.Printf("submit job OK: job_id=%d status=%s", result.Job.ID, result.Job.Status)

	// Arsipkan file segera, di luar UI thread, supaya tetap terjadi
	// walaupun dialog sudah ditutup guru.
	dest := filepath.Join(cfg.SentDir, fmt.Sprintf("%d_%s", time.Now().Unix(), item.FileName))
	if err := os.Rename(item.Path, dest); err != nil {
		log.Printf("gagal memindahkan file ke arsip: %v", err)
	}

	token := tokenFromURL(result.Job.StatusURL)
	printerName := result.Job.PrinterName

	ui.onUI("submit ok", func() {
		ui.cancelBtn.SetText("Tutup")
		ui.cancelBtn.SetEnabled(true)
		ui.printBtn.SetVisible(false)
		ui.setStatus("Menunggu di antrean",
			"Job sudah diterima server. Menunggu komputer printer ("+printerName+") mengambilnya.",
			colorWait, true)
	})

	if token == "" {
		log.Printf("status_url tidak berisi token, polling dilewati: %q", result.Job.StatusURL)
		return
	}
	ui.poll(cfg.ServerURL, token, printerName)
}

// poll cek status job tiap 3 detik sampai selesai/gagal, dialog ditutup,
// atau 10 menit berlalu.
func (ui *printUI) poll(serverURL, token, printerName string) {
	defer recoverLog("poll status")
	deadline := time.Now().Add(10 * time.Minute)
	last := "pending"
	fails := 0

	for time.Now().Before(deadline) {
		time.Sleep(3 * time.Second)
		if ui.closed.Load() {
			return
		}

		st, err := FetchJobStatus(serverURL, token)
		if err != nil {
			fails++
			log.Printf("poll status gagal (%d): %v", fails, err)
			if fails == 3 {
				ui.onUI("poll koneksi", func() {
					_ = ui.statusDetail.SetText("Koneksi ke server terputus, mencoba lagi... (job tetap aman di antrean)")
				})
			}
			continue
		}
		fails = 0
		if st.Status == last {
			continue
		}
		last = st.Status
		log.Printf("status job berubah: %s", st.Status)

		s := *st
		ui.onUI("poll update", func() { ui.applyStatus(s, printerName) })
		if s.Status == "done" || s.Status == "failed" {
			return
		}
	}

	ui.onUI("poll timeout", func() {
		ui.setStatus("Belum ada kabar dari printer",
			"Job masih di antrean. Komputer printer mungkin sedang mati. Anda boleh menutup jendela ini.",
			colorWait, false)
	})
}

func (ui *printUI) applyStatus(st JobStatus, printerName string) {
	switch st.Status {
	case "pending":
		ui.setStatus("Menunggu di antrean",
			"Menunggu komputer printer ("+printerName+") mengambil job.", colorWait, true)
	case "printing":
		ui.setStatus("Sedang dicetak",
			"Dokumen sedang dikirim ke printer "+printerName+".", colorWait, true)
	case "done":
		ui.setStatus("Selesai dicetak",
			"Silakan ambil hasil cetak di printer "+printerName+".", colorOK, false)
		ui.cancelBtn.SetText("Selesai")
	case "failed":
		reason := strings.TrimSpace(st.ErrorMessage)
		if reason == "" {
			reason = "Tidak ada keterangan dari komputer printer."
		}
		ui.setStatus("Gagal dicetak", truncate(reason, 160), colorErr, false)
		ui.cancelBtn.SetText("Tutup")
	default:
		ui.setStatus("Status: "+st.Status, "", colorWait, false)
	}
}

// tokenFromURL ambil segmen terakhir dari status_url (token 64 hex).
func tokenFromURL(u string) string {
	if i := strings.IndexAny(u, "?#"); i >= 0 {
		u = u[:i]
	}
	u = strings.TrimRight(u, "/")
	return u[strings.LastIndex(u, "/")+1:]
}
