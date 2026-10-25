package main

import (
	"fmt"
	"html/template"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

const baseCSS = `
:root {
  --accent:#0078D4; --bg:#F3F3F3; --card:rgba(255,255,255,0.75); --text:#1A1A1A;
  --dim:#5c5c5c; --border:rgba(0,0,0,0.08);
}
@media (prefers-color-scheme: dark) {
  :root { --bg:#202020; --card:rgba(255,255,255,0.06); --text:#F3F3F3; --dim:#b0b0b0; --border:rgba(255,255,255,0.1); }
}
* { box-sizing: border-box; }
body { font-family:"Segoe UI Variable","Segoe UI",system-ui,sans-serif; background:var(--bg); color:var(--text);
  max-width:460px; margin:40px auto; padding:0 20px; }
h1 { font-size:18px; display:flex; align-items:center; gap:8px; }
.card { background:var(--card); backdrop-filter:blur(20px); border:1px solid var(--border); border-radius:12px;
  padding:20px; box-shadow:0 1px 3px rgba(0,0,0,.08),0 4px 14px rgba(0,0,0,.05); }
label { display:block; font-size:12px; color:var(--dim); margin:12px 0 4px; }
input, select { width:100%; padding:9px 10px; border-radius:6px; border:1px solid var(--border);
  background:var(--bg); color:var(--text); font-size:14px; }
button { margin-top:18px; width:100%; padding:10px; background:var(--accent); color:#fff; border:none;
  border-radius:6px; font-size:14px; font-weight:500; cursor:pointer; }
button:hover { background:#106EBE; }
.printer-opt { display:flex; align-items:center; gap:8px; padding:10px; border:1px solid var(--border);
  border-radius:8px; margin-bottom:8px; cursor:pointer; }
.printer-opt input { width:auto; }
.dot { width:7px; height:7px; border-radius:50%; display:inline-block; }
.dot-on { background:#0F9D58; } .dot-off { background:#D13438; }
.muted { color:var(--dim); font-size:13px; }
`

func (a *App) handleSetup(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		r.ParseForm()
		a.cfg.ServerURL = r.FormValue("server_url")
		a.cfg.Pin = r.FormValue("pin")
		_ = a.cfg.Save()
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	tmpl := template.Must(template.New("setup").Parse(`
<!DOCTYPE html><html lang="id"><head><meta charset="UTF-8"><title>Setup MARSA Print</title>
<style>` + baseCSS + `</style></head><body>
<h1>🖨️ Setup MARSA Print</h1>
<p class="muted">Diisi sekali saja. PIN akan disimpan di laptop ini dan tidak diminta lagi.</p>
<div class="card">
<form method="POST">
  <label>URL Server</label>
  <input type="text" name="server_url" placeholder="https://chat.marsa9.com/printserver-ci4/public" value="{{.ServerURL}}" required>
  <label>PIN Guru</label>
  <input type="password" name="pin" value="{{.Pin}}" required>
  <button type="submit">Simpan</button>
</form>
</div>
</body></html>`))
	_ = tmpl.Execute(w, a.cfg)
}

func (a *App) handleHome(w http.ResponseWriter, r *http.Request) {
	if !a.cfg.IsReady() {
		http.Redirect(w, r, "/setup", http.StatusSeeOther)
		return
	}
	pending := a.queue.All()

	tmpl := template.Must(template.New("home").Parse(`
<!DOCTYPE html><html lang="id"><head><meta charset="UTF-8"><title>MARSA Print</title>
<style>` + baseCSS + `</style></head><body>
<h1>🖨️ MARSA Print</h1>
<div class="card">
<p>Siap. Cetak dari Word/Excel/PDF pilih printer <strong>"MARSA Print Server"</strong>, simpan filenya - jendela pilih printer tujuan akan muncul otomatis.</p>
{{if .Pending}}
<p class="muted">File menunggu dipilih printer-nya:</p>
<ul>{{range .Pending}}<li><a href="/print?id={{.ID}}">{{.FileName}}</a></li>{{end}}</ul>
{{else}}
<p class="muted">Tidak ada file yang menunggu saat ini.</p>
{{end}}
<p class="muted"><a href="/setup">Ubah setting</a></p>
</div>
</body></html>`))
	_ = tmpl.Execute(w, struct{ Pending []*PendingFile }{pending})
}

func (a *App) handlePrintPage(w http.ResponseWriter, r *http.Request) {
	if !a.cfg.IsReady() {
		http.Redirect(w, r, "/setup", http.StatusSeeOther)
		return
	}
	id := r.URL.Query().Get("id")
	item, ok := a.queue.Get(id)
	if !ok {
		fmt.Fprintf(w, "File tidak ditemukan atau sudah diproses.")
		return
	}

	printers, err := FetchPrinters(a.cfg.ServerURL)
	errMsg := ""
	if err != nil {
		errMsg = err.Error()
	}

	tmpl := template.Must(template.New("print").Parse(`
<!DOCTYPE html><html lang="id"><head><meta charset="UTF-8"><title>Pilih Printer</title>
<style>` + baseCSS + `</style></head><body>
<h1>🖨️ Cetak Dokumen</h1>
<div class="card">
<p><strong>{{.File.FileName}}</strong></p>
{{if .Err}}<p class="muted" style="color:#D13438;">Gagal ambil daftar printer: {{.Err}}</p>{{end}}
<form method="POST" action="/submit">
  <input type="hidden" name="id" value="{{.File.ID}}">
  <label>Pilih Printer</label>
  {{range .Printers}}
  <label class="printer-opt">
    <input type="radio" name="printer_id" value="{{.ID}}" required>
    <span class="dot dot-on"></span>
    {{.Nama}} <span class="muted">({{.Lokasi}})</span>
  </label>
  {{end}}
  <label>Jumlah Copy</label>
  <input type="number" name="copies" value="1" min="1" max="50">
  <button type="submit">Cetak Sekarang</button>
</form>
</div>
</body></html>`))
	_ = tmpl.Execute(w, struct {
		File     *PendingFile
		Printers []TeacherPrinter
		Err      string
	}{item, printers, errMsg})
}

func (a *App) handleSubmit(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	id := r.FormValue("id")
	printerID, _ := strconv.Atoi(r.FormValue("printer_id"))
	copies, _ := strconv.Atoi(r.FormValue("copies"))
	if copies < 1 {
		copies = 1
	}

	item, ok := a.queue.Get(id)
	if !ok {
		fmt.Fprint(w, "File tidak ditemukan.")
		return
	}

	result, err := SubmitJob(a.cfg.ServerURL, a.cfg.Pin, item.Path, printerID, copies)

	tmpl := template.Must(template.New("result").Parse(`
<!DOCTYPE html><html lang="id"><head><meta charset="UTF-8"><title>Status Cetak</title>
<style>` + baseCSS + `</style></head><body>
<h1>🖨️ Status Cetak</h1>
<div class="card">
{{if .Err}}
  <p style="color:#D13438;"><strong>Gagal:</strong> {{.Err}}</p>
{{else}}
  <p style="color:#0F9D58;"><strong>Berhasil masuk antrean cetak.</strong></p>
  <p class="muted">Printer: {{.Result.Job.PrinterName}}</p>
{{end}}
<p class="muted"><a href="/">Kembali</a></p>
</div>
</body></html>`))

	errMsg := ""
	if err != nil {
		errMsg = err.Error()
	} else {
		a.queue.Remove(id)
		// Pindahkan file ke folder arsip supaya tidak terdeteksi ulang
		dest := filepath.Join(a.cfg.SentDir, fmt.Sprintf("%d_%s", time.Now().Unix(), item.FileName))
		_ = os.Rename(item.Path, dest)
	}

	_ = tmpl.Execute(w, struct {
		Err    string
		Result *SubmitResult
	}{errMsg, result})
}
