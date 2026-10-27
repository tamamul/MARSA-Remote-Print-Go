package main

import (
	"fmt"
	"html/template"
	"net/http"
	"strconv"
	"time"
)

const dashboardHTML = `
<!DOCTYPE html>
<html lang="id">
<head>
<meta charset="UTF-8">
<title>MARSA Print Agent</title>
<style>
  body { font-family: system-ui, sans-serif; max-width: 800px; margin: 30px auto; padding: 0 16px; color: #1a1a1a; }
  h1 { font-size: 20px; }
  .card { border: 1px solid #ddd; border-radius: 8px; padding: 16px; margin-bottom: 16px; }
  .ok { color: #16a34a; font-weight: 600; }
  .warn { color: #dc2626; font-weight: 600; }
  table { width: 100%; border-collapse: collapse; font-size: 14px; }
  td, th { text-align: left; padding: 6px 4px; border-bottom: 1px solid #eee; }
  .status-done { color: #16a34a; }
  .status-failed { color: #dc2626; }
  .status-printing { color: #d97706; }
  input { width: 100%; padding: 6px; margin-bottom: 10px; box-sizing: border-box; }
  button { padding: 8px 16px; background: #1a1a1a; color: white; border: none; border-radius: 6px; cursor: pointer; }
  .msg { padding: 8px 12px; border-radius: 6px; margin-bottom: 12px; }
  .msg-ok { background: #dcfce7; color: #166534; }
</style>
</head>
<body>
<h1>🖨️ MARSA Print Agent</h1>

{{if .Saved}}<div class="msg msg-ok">Setting tersimpan. Diterapkan langsung tanpa perlu restart.</div>{{end}}

<div class="card">
  <strong>Status:</strong>
  {{if .IsHealthy}}<span class="ok">● Berjalan normal</span>{{else}}<span class="warn">● Belum ada heartbeat terbaru</span>{{end}}
  <br>Jalan sejak: {{.StartedAt}}
  <br>Heartbeat terakhir: {{.LastHeartbeat}}
  <br>Polling terakhir: {{.LastPoll}}
</div>

<div class="card">
  <strong>Riwayat Job Terakhir</strong>
  <table>
    <tr><th>Waktu</th><th>Job</th><th>Printer</th><th>Status</th><th>Keterangan</th></tr>
    {{range .RecentJobs}}
    <tr>
      <td>{{.Timestamp.Format "15:04:05"}}</td>
      <td>#{{.JobID}}</td>
      <td>{{.Printer}}</td>
      <td class="status-{{.Status}}">{{.Status}}</td>
      <td>{{.Message}}</td>
    </tr>
    {{else}}
    <tr><td colspan="5">Belum ada job masuk</td></tr>
    {{end}}
  </table>
</div>

<div class="card">
  <strong>Setting</strong>
  <form method="POST" action="/settings">
    <label>URL Server</label>
    <input type="text" name="server_url" value="{{.Cfg.ServerURL}}">
    <label>Token Agent</label>
    <input type="text" name="agent_token" value="{{.Cfg.AgentToken}}">
    <label>Interval Polling (detik)</label>
    <input type="number" name="poll_interval_sec" value="{{.Cfg.PollIntervalSec}}">
    <label>Interval Heartbeat (detik)</label>
    <input type="number" name="heartbeat_sec" value="{{.Cfg.HeartbeatSec}}">
    <label>Print Settings (SumatraPDF)</label>
    <input type="text" name="print_settings" value="{{.Cfg.PrintSettings}}" placeholder="fit">
    <button type="submit">Simpan</button>
  </form>
</div>

</body>
</html>
`

type dashboardData struct {
	Saved         bool
	IsHealthy     bool
	StartedAt     string
	LastHeartbeat string
	LastPoll      string
	RecentJobs    []JobEvent
	Cfg           *Config
}

// startWebUI jalankan dashboard lokal di 127.0.0.1:<port>.
// Sengaja cuma bind ke localhost (bukan 0.0.0.0) supaya tidak bisa
// diakses dari luar PC itu — tidak perlu auth tambahan.
func (p *program) startWebUI() {
	tmpl := template.Must(template.New("dashboard").Parse(dashboardHTML))

	mux := http.NewServeMux()

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		snap := p.state.Snapshot()
		p.cfgMu.RLock()
		cfg := *p.cfg
		p.cfgMu.RUnlock()

		data := dashboardData{
			Saved:         r.URL.Query().Get("saved") == "1",
			IsHealthy:     !snap.LastHeartbeat.IsZero(),
			StartedAt:     snap.StartedAt.Format("2006-01-02 15:04:05"),
			LastHeartbeat: formatTimeOrDash(snap.LastHeartbeat),
			LastPoll:      formatTimeOrDash(snap.LastPoll),
			RecentJobs:    snap.RecentJobs,
			Cfg:           &cfg,
		}
		_ = tmpl.Execute(w, data)
	})

	mux.HandleFunc("/settings", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Redirect(w, r, "/", http.StatusSeeOther)
			return
		}
		r.ParseForm()

		p.cfgMu.Lock()
		p.cfg.ServerURL = r.FormValue("server_url")
		p.cfg.AgentToken = r.FormValue("agent_token")
		if v, err := strconv.Atoi(r.FormValue("poll_interval_sec")); err == nil && v > 0 {
			p.cfg.PollIntervalSec = v
		}
		if v, err := strconv.Atoi(r.FormValue("heartbeat_sec")); err == nil && v > 0 {
			p.cfg.HeartbeatSec = v
		}
		if v := r.FormValue("print_settings"); v != "" {
			p.cfg.PrintSettings = v
		}
		cfgCopy := *p.cfg
		p.cfgMu.Unlock()

		// Simpan ke file + bikin ulang API client pakai setting baru,
		// jadi langsung aktif tanpa perlu restart service
		_ = cfgCopy.Save(p.configPath)
		p.apiMu.Lock()
		p.api = NewAPIClient(&cfgCopy)
		p.apiMu.Unlock()

		http.Redirect(w, r, "/?saved=1", http.StatusSeeOther)
	})

	addr := fmt.Sprintf("127.0.0.1:%d", p.cfg.DashboardPort)
	go func() {
		if err := http.ListenAndServe(addr, mux); err != nil {
			logger.Warningf("dashboard web gagal start di %s: %v", addr, err)
		}
	}()
}

func formatTimeOrDash(t time.Time) string {
	if t.IsZero() {
		return "-"
	}
	return t.Format("2006-01-02 15:04:05")
}
