package workspace

import (
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

// CreateRunDirectory creates a temporary, isolated folder for a specific test run and agent
func CreateRunDirectory(runID string, agentID string) (string, error) {
	slog.Info("Creating run directory", "run_id", runID, "agent_id", agentID)

	dir := filepath.Join(".", "runs", fmt.Sprintf("%s_%s", runID, agentID))

	err := os.MkdirAll(dir, 0755)
	if err != nil {
		slog.Error("Failed to create run directory", "run_id", runID, "agent_id", agentID, "error", err)
		return "", err
	}

	slog.Info("Successfully created run directory", "path", dir)
	return dir, nil
}

// DownloadFile downloads a file from the given url to the destPath.
// A 5-minute timeout is applied and the response body is limited to 500MB
// to prevent hangs and disk exhaustion from a slow or malicious server.
func DownloadFile(url string, destPath string) error {
	slog.Info("Starting download", "url", url, "dest", destPath)

	out, err := os.Create(destPath)
	if err != nil {
		slog.Error("Failed to create destination file", "path", destPath, "error", err)
		return err
	}
	defer out.Close()

	client := &http.Client{Timeout: 5 * time.Minute}
	resp, err := client.Get(url)
	if err != nil {
		slog.Error("Failed to issue GET request", "url", url, "error", err)
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		err := fmt.Errorf("bad status: %s", resp.Status)
		slog.Error("Download failed with non-OK status", "url", url, "status", resp.Status)
		return err
	}

	const maxDownloadSize = 500 * 1024 * 1024 // 500 MB
	bytesWritten, err := io.Copy(out, io.LimitReader(resp.Body, maxDownloadSize))
	if err != nil {
		slog.Error("Failed to write data to file", "path", destPath, "error", err)
		return err
	}

	slog.Info("Successfully downloaded file", "dest", destPath, "bytes", bytesWritten)
	return nil
}

// SyncPlugins checks if required .jar files exist and downloads them if not
func SyncPlugins(pluginURLs []string, jmeterExtPath string) error {
	slog.Info("Syncing plugins", "count", len(pluginURLs), "extPath", jmeterExtPath)

	// Ensure the JMeter ext directory exists
	if err := os.MkdirAll(jmeterExtPath, 0755); err != nil {
		slog.Error("Failed to create JMeter extension path", "path", jmeterExtPath, "error", err)
		return err
	}

	for _, rawURL := range pluginURLs {
		filename := filepath.Base(rawURL)
		destPath := filepath.Join(jmeterExtPath, filename)

		// Check if file exists
		if _, err := os.Stat(destPath); err == nil {
			slog.Info("Plugin already exists, skipping download", "plugin", filename)
			continue
		} else if !os.IsNotExist(err) {
			slog.Error("Error checking plugin existence", "path", destPath, "error", err)
			return err
		}

		// Doesn't exist, download it
		slog.Info("Plugin missing, downloading", "plugin", filename)
		if err := DownloadFile(rawURL, destPath); err != nil {
			slog.Error("Failed to download plugin", "plugin", filename, "error", err)
			return err
		}
	}

	slog.Info("Plugin sync completed successfully")
	return nil
}
