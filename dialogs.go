package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/lxn/walk"
	. "github.com/lxn/walk/declarative"
	"github.com/lxn/win"
)

// bringToFront: dialog dari proses background (watcher) gampang "tenggelam"
// di belakang Word karena Windows melarang app non-aktif merebut fokus.
// Dibuat TOPMOST supaya pasti kelihatan walau tidak bisa merebut fokus.
func bringToFront(dlg *walk.Dialog) {
	h := dlg.Handle()
	win.SetWindowPos(h, win.HWND_TOPMOST, 0, 0, 0, 0, win.SWP_NOMOVE|win.SWP_NOSIZE|win.SWP_SHOWWINDOW)
	win.SetForegroundWindow(h)
}

// runOnTop menggantikan Dialog{...}.Run(nil): dialog dibuat dulu, lalu
// dipasang handler Starting supaya langsung dibawa ke depan.
func runOnTop(d Dialog, dlg **walk.Dialog) {
	if err := d.Create(nil); err != nil {
		log.Printf("gagal membuat dialog: %v", err)
		walk.MsgBox(nil, "Error", "Gagal membuat jendela: "+err.Error(), walk.MsgBoxIconError|walk.MsgBoxStyle(win.MB_TOPMOST))
		return
	}
	(*dlg).Starting().Attach(func() { bringToFront(*dlg) })
	(*dlg).Run()
}

// showSetupDialog dipanggil sekali di awal (kalau config belum diisi) atau
// lewat menu tray "Setting". Jendela native biasa, OK-nya nyimpen config.
func showSetupDialog(owner walk.Form, cfg *Config) {
	defer owner.SetVisible(false) // pengaman: jendela utama harus tetap tersembunyi
	var dlg *walk.Dialog
	var serverEdit, pinEdit *walk.LineEdit

	runOnTop(Dialog{
		AssignTo: &dlg,
		Title:    "Setup MARSA Print",
		MinSize:  Size{Width: 360, Height: 180},
		Layout:   VBox{},
		Children: []Widget{
			Label{Text: "URL Server:"},
			LineEdit{AssignTo: &serverEdit, Text: cfg.ServerURL},
			Label{Text: "PIN Guru (5 digit):"},
			LineEdit{AssignTo: &pinEdit, Text: cfg.Pin, PasswordMode: true},
			Composite{
				Layout: HBox{},
				Children: []Widget{
					HSpacer{},
					PushButton{
						Text: "Simpan",
						OnClicked: func() {
							cfg.ServerURL = serverEdit.Text()
							cfg.Pin = pinEdit.Text()
							_ = cfg.Save()
							dlg.Accept()
						},
					},
					PushButton{Text: "Batal", OnClicked: func() { dlg.Cancel() }},
				},
			},
		},
	}, &dlg)
}

// showPrintDialog muncul otomatis tiap ada PDF baru terdeteksi di folder.
// Guru pilih printer dari dropdown + jumlah copy, lalu klik Cetak.
func showPrintDialog(owner *walk.MainWindow, cfg *Config, item *PendingFile, printers []TeacherPrinter) {
	defer owner.SetVisible(false) // pengaman: jendela utama harus tetap tersembunyi
	var dlg *walk.Dialog
	var combo *walk.ComboBox
	var copiesCombo *walk.ComboBox
	var statusLbl *walk.Label
	var printBtn, cancelBtn *walk.PushButton

	names := make([]string, len(printers))
	for i, p := range printers {
		names[i] = fmt.Sprintf("%s  (%s)", p.Nama, p.Lokasi)
	}

	runOnTop(Dialog{
		AssignTo: &dlg,
		Title:    "Cetak Dokumen",
		MinSize:  Size{Width: 380, Height: 220},
		Layout:   VBox{},
		Children: []Widget{
			Label{Text: item.FileName, Font: Font{Family: "Segoe UI", PointSize: 10, Bold: true}},
			Label{Text: "Pilih Printer:"},
			ComboBox{AssignTo: &combo, Model: names, CurrentIndex: 0},
			Label{Text: "Jumlah Copy (maks 10):"},
			ComboBox{AssignTo: &copiesCombo, Model: []string{"1", "2", "3", "4", "5", "6", "7", "8", "9", "10"}, CurrentIndex: 0},
			Label{AssignTo: &statusLbl, Text: ""},
			Composite{
				Layout: HBox{},
				Children: []Widget{
					HSpacer{},
					PushButton{
						AssignTo: &printBtn,
						Text:     "Cetak",
						OnClicked: func() {
							idx := combo.CurrentIndex()
							if idx < 0 {
								return
							}
							printerID, err := strconv.Atoi(printers[idx].ID)
							if err != nil {
								walk.MsgBox(dlg, "Error", "ID printer tidak valid.", walk.MsgBoxIconError)
								return
							}
							copies := copiesCombo.CurrentIndex() + 1

							printBtn.SetEnabled(false)
							cancelBtn.SetEnabled(false)
							statusLbl.SetText("Mengirim ke server...")

							go submitAndClose(owner, dlg, statusLbl, printBtn, cancelBtn, cfg, item, printerID, copies)
						},
					},
					PushButton{AssignTo: &cancelBtn, Text: "Batal", OnClicked: func() { dlg.Cancel() }},
				},
			},
		},
	}, &dlg)
}

// submitAndClose jalan di goroutine terpisah (biar jendela tidak freeze
// pas upload), lalu balik ke UI thread lewat Synchronize buat update dialog.
func submitAndClose(owner *walk.MainWindow, dlg *walk.Dialog, statusLbl *walk.Label, printBtn, cancelBtn *walk.PushButton, cfg *Config, item *PendingFile, printerID, copies int) {
	log.Printf("submit job: file=%s printer_id=%d copies=%d", item.FileName, printerID, copies)
	result, err := SubmitJob(cfg.ServerURL, cfg.Pin, item.Path, printerID, copies)
	if err != nil {
		log.Printf("submit job GAGAL: %v", err)
	} else {
		log.Printf("submit job OK: job_id=%d status=%s", result.Job.ID, result.Job.Status)
	}

	owner.Synchronize(func() {
		// Wajib: kalau dialog ditutup lewat tombol X selama upload berjalan,
		// widget sudah dibuang dan SetText di bawah bisa panic.
		defer recoverLog("submit selesai")
		if err != nil {
			statusLbl.SetText("Gagal: " + err.Error())
			printBtn.SetEnabled(true)
			cancelBtn.SetEnabled(true)
			return
		}

		dest := filepath.Join(cfg.SentDir, fmt.Sprintf("%d_%s", time.Now().Unix(), item.FileName))
		if err := os.Rename(item.Path, dest); err != nil {
			log.Printf("gagal memindahkan file ke arsip: %v", err)
		}

		walk.MsgBox(dlg, "Berhasil",
			"Dokumen masuk antrean cetak ke "+result.Job.PrinterName+".\n\n"+
				"Catatan: ini baru status ANTREAN, bukan konfirmasi sudah tercetak - "+
				"terutama untuk printer di lokasi lain, prosesnya masih nunggu Agent di sana ambil & cetak job-nya.",
			walk.MsgBoxIconInformation)
		dlg.Accept()
	})
}
