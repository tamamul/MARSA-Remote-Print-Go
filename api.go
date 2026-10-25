package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

type TeacherPrinter struct {
	ID       int    `json:"id"`
	Nama     string `json:"nama_printer"`
	Lokasi   string `json:"lokasi"`
	Online   bool   `json:"online"`
}

// FetchPrinters panggil GET {server}/api/teacher/printers?pin=xxx
// (endpoint baru, PIN-authenticated - lihat catatan di server)
func FetchPrinters(serverURL, pin string) ([]TeacherPrinter, error) {
	client := &http.Client{Timeout: 15 * time.Second}
	url := fmt.Sprintf("%s/api/teacher/printers?pin=%s", serverURL, pin)

	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("server balas status %d: %s", resp.StatusCode, string(body))
	}

	var printers []TeacherPrinter
	if err := json.Unmarshal(body, &printers); err != nil {
		return nil, err
	}
	return printers, nil
}

type SubmitResult struct {
	OK      bool   `json:"ok"`
	Message string `json:"message"`
	Job     struct {
		ID           int    `json:"id"`
		Status       string `json:"status"`
		PrinterName  string `json:"printer_name"`
		StatusURL    string `json:"status_url"`
	} `json:"job"`
}

// SubmitJob upload PDF ke /api/teacher/jobs (multipart form, sama persis
// dengan yang sudah terbukti jalan lewat curl)
func SubmitJob(serverURL, pin, filePath string, printerID, copies int) (*SubmitResult, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	_ = writer.WriteField("pin", pin)
	_ = writer.WriteField("printer_id", fmt.Sprintf("%d", printerID))
	_ = writer.WriteField("jumlah_copy", fmt.Sprintf("%d", copies))

	part, err := writer.CreateFormFile("pdf", filepath.Base(filePath))
	if err != nil {
		return nil, err
	}
	if _, err := io.Copy(part, file); err != nil {
		return nil, err
	}
	writer.Close()

	req, err := http.NewRequest(http.MethodPost, serverURL+"/api/teacher/jobs", &buf)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	var result SubmitResult
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("balasan server tidak terbaca: %s", string(respBody))
	}
	if !result.OK {
		return &result, fmt.Errorf("server menolak: %s", result.Message)
	}
	return &result, nil
}
