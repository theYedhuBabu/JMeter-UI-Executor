package cleanup

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
)

// ArchiveResults zips the .jtl and jmeter.log files in the isolated run directory.
func ArchiveResults(runID string) (string, error) {
	slog.Info("Archiving results", "run_id", runID)
	runDir := filepath.Join(".", "runs", runID)
	zipPath := filepath.Join(runDir, "results.zip")

	zipFile, err := os.Create(zipPath)
	if err != nil {
		slog.Error("Failed to create zip file", "run_id", runID, "error", err)
		return "", err
	}
	defer zipFile.Close()

	archive := zip.NewWriter(zipFile)
	defer archive.Close()

	filesToZip := []string{"results.jtl", "jmeter.log"}

	for _, fileName := range filesToZip {
		filePath := filepath.Join(runDir, fileName)
		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			slog.Warn("File not found, skipping archive for this file", "file", fileName)
			continue
		}

		f, err := os.Open(filePath)
		if err != nil {
			slog.Error("Failed to open file for archiving", "file", filePath, "error", err)
			continue
		}

		w, err := archive.Create(fileName)
		if err != nil {
			f.Close()
			return "", err
		}

		if _, err := io.Copy(w, f); err != nil {
			f.Close()
			return "", err
		}
		f.Close()
	}

	slog.Info("Successfully archived results", "zip_path", zipPath)
	return zipPath, nil
}

// UploadResults performs a multipart HTTP POST to upload the zip file.
func UploadResults(zipPath string, targetURL string) error {
	slog.Info("Uploading results", "zip_path", zipPath, "target_url", targetURL)

	file, err := os.Open(zipPath)
	if err != nil {
		slog.Error("Failed to open zip file for uploading", "zip_path", zipPath, "error", err)
		return err
	}
	defer file.Close()

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	part, err := writer.CreateFormFile("file", filepath.Base(zipPath))
	if err != nil {
		return err
	}

	if _, err := io.Copy(part, file); err != nil {
		return err
	}

	if err := writer.Close(); err != nil {
		return err
	}

	req, err := http.NewRequest("POST", targetURL, body)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		slog.Error("Failed to execute HTTP POST for upload", "error", err)
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		err := fmt.Errorf("upload failed with HTTP status %s", resp.Status)
		slog.Error("Upload HTTP response was not OK", "status", resp.Status)
		return err
	}

	slog.Info("Successfully uploaded results to Hub", "target_url", targetURL)
	return nil
}

// WipeWorkspace completely deletes the run directory.
func WipeWorkspace(runID string) error {
	slog.Info("Wiping workspace", "run_id", runID)
	runDir := filepath.Join(".", "runs", runID)

	if err := os.RemoveAll(runDir); err != nil {
		slog.Error("Failed to completely wipe workspace", "run_id", runID, "error", err)
		return err
	}

	slog.Info("Workspace successfully wiped", "run_id", runID)
	return nil
}
