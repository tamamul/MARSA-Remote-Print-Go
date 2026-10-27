package main

import (
	"log"
	"math/rand"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/kardianos/service"
)

var logger service.Logger

type program struct {
	cfg        *Config
	cfgMu      sync.RWMutex
	api        *APIClient
	apiMu      sync.RWMutex
	state      *RuntimeState
	baseDir    string // folder tempat agent.exe berada (bukan CWD saat jalan sbg service)
	configPath string
	stopCh     chan struct{}
}

func (p *program) getCfg() Config {
	p.cfgMu.RLock()
	defer p.cfgMu.RUnlock()
	return *p.cfg
}

func (p *program) getAPI() *APIClient {
	p.apiMu.RLock()
	defer p.apiMu.RUnlock()
	return p.api
}

func (p *program) Start(s service.Service) error {
	p.stopCh = make(chan struct{})
	p.state = NewRuntimeState()
	go p.run()
	return nil
}

func (p *program) Stop(s service.Service) error {
	close(p.stopCh)
	return nil
}

func (p *program) run() {
	p.startWebUI()
	go p.heartbeatLoop()
	p.pollLoop()
}

// pollLoop cek job baru tiap N detik + jitter kecil biar tidak barengan
// dengan agent lain saat request ke server
func (p *program) pollLoop() {
	tempDir := os.TempDir()

	for {
		select {
		case <-p.stopCh:
			return
		default:
		}

		cfg := p.getCfg()
		jobs, err := p.getAPI().FetchJobs()
		p.state.MarkPoll()

		if err != nil {
			// Server lagi error/tidak kebaca - jangan proses apa pun di
			// putaran ini, tunggu interval berikutnya lalu coba lagi.
			logger.Warningf("gagal ambil job, dilewati putaran ini: %v", err)
		} else {
			for _, job := range jobs {
				p.processJob(job, cfg.AgentToken, tempDir)
			}
		}

		jitter := time.Duration(rand.Intn(5)) * time.Second
		interval := time.Duration(cfg.PollIntervalSec)*time.Second + jitter
		time.Sleep(interval)
	}
}

func (p *program) processJob(job PrintJob, token string, tempDir string) {
	api := p.getAPI()
	_ = api.UpdateJobStatus(job.ID, "printing", "")
	p.state.AddJobEvent(JobEvent{JobID: job.ID, Printer: job.PrinterName, Status: "printing"})

	filePath, err := downloadFile(job.FileURL, tempDir, job.ID, token)
	if err != nil {
		logger.Errorf("job %d: gagal download: %v", job.ID, err)
		_ = api.UpdateJobStatus(job.ID, "failed", err.Error())
		p.state.AddJobEvent(JobEvent{JobID: job.ID, Printer: job.PrinterName, Status: "failed", Message: "gagal download: " + err.Error()})
		return
	}
	defer cleanupFile(filePath)

	cfg := p.getCfg()
	if err := printPDF(filePath, job.PrinterName, job.Copies, p.baseDir, cfg.PrintSettings); err != nil {
		logger.Errorf("job %d: gagal print: %v", job.ID, err)
		_ = api.UpdateJobStatus(job.ID, "failed", err.Error())
		p.state.AddJobEvent(JobEvent{JobID: job.ID, Printer: job.PrinterName, Status: "failed", Message: "gagal print: " + err.Error()})
		return
	}

	_ = api.UpdateJobStatus(job.ID, "done", "")
	p.state.AddJobEvent(JobEvent{JobID: job.ID, Printer: job.PrinterName, Status: "done", Message: "berhasil dicetak"})
	logger.Infof("job %d selesai dicetak ke %s", job.ID, job.PrinterName)
}

// heartbeatLoop lapor "masih hidup" secara terpisah dari polling job,
// intervalnya lebih santai (default 60 detik)
func (p *program) heartbeatLoop() {
	for {
		cfg := p.getCfg()
		select {
		case <-p.stopCh:
			return
		case <-time.After(time.Duration(cfg.HeartbeatSec) * time.Second):
			if err := p.getAPI().Heartbeat(); err != nil {
				logger.Warningf("heartbeat gagal: %v", err)
			} else {
				p.state.MarkHeartbeat()
			}
		}
	}
}

func main() {
	// Deteksi folder tempat agent.exe berada - PENTING karena saat jalan
	// sebagai Windows Service, working directory default-nya System32,
	// bukan folder agent.exe. Semua path (config.json, SumatraPDF.exe)
	// harus relatif ke sini, bukan ke CWD.
	exePath, err := os.Executable()
	if err != nil {
		log.Fatalf("gagal deteksi lokasi exe: %v", err)
	}
	baseDir := filepath.Dir(exePath)
	configPath := filepath.Join(baseDir, "config.json")

	cfg, err := loadConfig(configPath)
	if err != nil {
		log.Fatalf("gagal baca config.json: %v", err)
	}

	svcConfig := &service.Config{
		Name:        "MarsaPrintAgent",
		DisplayName: "MARSA Print Agent",
		Description: "Agent cetak jarak jauh untuk MARSA Remote Print",
		Option: service.KeyValue{
			"OnFailure":              "restart",
			"OnFailureDelayDuration": "5s",
			"OnFailureResetPeriod":   10,
		},
	}

	prg := &program{
		cfg:        cfg,
		api:        NewAPIClient(cfg),
		baseDir:    baseDir,
		configPath: configPath,
	}

	s, err := service.New(prg, svcConfig)
	if err != nil {
		log.Fatal(err)
	}

	logger, err = s.Logger(nil)
	if err != nil {
		log.Fatal(err)
	}

	// Jalankan sebagai: agent.exe install  → daftar sebagai service
	//                   agent.exe start    → jalankan service
	//                   agent.exe stop     → hentikan service
	//                   agent.exe uninstall → hapus service
	if len(os.Args) > 1 {
		err = service.Control(s, os.Args[1])
		if err != nil {
			log.Fatal(err)
		}
		return
	}

	err = s.Run()
	if err != nil {
		logger.Error(err)
	}
}
