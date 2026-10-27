package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

// downloadFile ambil file job dari server, simpan ke folder temp.
// Pakai client dengan timeout supaya tidak nyangkut kalau server lemot.
// Endpoint file butuh token juga (sama seperti endpoint lain), jadi wajib
// dikirim lewat header Authorization, bukan cuma GET polos.
func downloadFile(url string, destDir string, jobID int, token string) (string, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+token)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("gagal download file, status %d", resp.StatusCode)
	}

	destPath := filepath.Join(destDir, fmt.Sprintf("job_%d.pdf", jobID))
	out, err := os.Create(destPath)
	if err != nil {
		return "", err
	}
	defer out.Close()

	if _, err := io.Copy(out, resp.Body); err != nil {
		return "", err
	}
	return destPath, nil
}

// printPDF kirim file PDF ke printer tertentu secara silent (tanpa dialog)
// pakai SumatraPDF.exe. baseDir = folder tempat agent.exe berada (bukan CWD
// saat ini, karena Windows Service biasanya start dari System32).
func printPDF(filePath string, printerName string, copies int, baseDir string, printSettings string) error {
	sumatraPath := filepath.Join(baseDir, "SumatraPDF.exe")

	for i := 0; i < copies; i++ {
		cmd := exec.Command(sumatraPath,
			"-print-to", printerName,
			"-print-settings", printSettings,
			"-silent",
			filePath,
		)
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("gagal print copy ke-%d: %w", i+1, err)
		}
	}
	return nil
}

// cleanupFile hapus file job setelah selesai dicetak (jaga privasi + hemat disk)
func cleanupFile(path string) {
	_ = os.Remove(path)
}
