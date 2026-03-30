package network

import (
	"encoding/json"
	"jmeter-agent/executor"
	"jmeter-agent/models"
	"jmeter-agent/workspace"
	"jmeter-agent/xmlparser"
	"log/slog"
	"math"
	"net/url"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

type Client struct {
	hubURL  string
	agentID string
	conn    *websocket.Conn
	mu      sync.Mutex
	done    chan struct{}
}

func NewClient(hubURL string, agentID string) *Client {
	return &Client{
		hubURL:  hubURL,
		agentID: agentID,
		done:    make(chan struct{}),
	}
}

func (c *Client) Connect() error {
	var err error
	maxRetries := 10
	baseDelay := 1 * time.Second

	for i := 0; i < maxRetries; i++ {
		c.mu.Lock()
		c.conn, _, err = websocket.DefaultDialer.Dial(c.hubURL, nil)
		c.mu.Unlock()

		if err == nil {
			slog.Info("Connected to Hub successfully", "url", c.hubURL)
			return nil
		}

		delay := time.Duration(math.Pow(2, float64(i))) * baseDelay
		slog.Warn("Connection failed, retrying", "error", err, "delay", delay)
		time.Sleep(delay)
	}
	return err
}

func (c *Client) Listen() {
	defer c.Close()
	for {
		// Exit gracefully when Close() has been called
		select {
		case <-c.done:
			return
		default:
		}

		c.mu.Lock()
		conn := c.conn
		c.mu.Unlock()

		if conn == nil {
			if err := c.Connect(); err != nil {
				slog.Error("Failed to reconnect", "error", err)
				time.Sleep(5 * time.Second)
				continue
			}
			continue
		}

		_, message, err := conn.ReadMessage()
		if err != nil {
			slog.Error("Read error, will attempt to reconnect", "error", err)
			c.mu.Lock()
			if c.conn != nil {
				c.conn.Close()
				c.conn = nil
			}
			c.mu.Unlock()
			continue
		}

		var cmd models.CommandMessage
		if err := json.Unmarshal(message, &cmd); err != nil {
			slog.Error("Failed to unmarshal command", "error", err, "message", string(message))
			continue
		}

		switch cmd.Action {
		case models.ActionStart:
			slog.Info("Received start command", "run_id", cmd.RunID)
			go c.startTest(&cmd)
		case models.ActionStop:
			slog.Info("Received stop command", "run_id", cmd.RunID)
			go c.stopTest(&cmd)
		case models.ActionPing:
			slog.Debug("Received ping command")
		default:
			// Ignore completely empty actions, as they are likely status/log broadcasts from the Hub
			if cmd.Action != "" {
				slog.Warn("Unknown command action", "action", cmd.Action)
			}
		}
	}
}

func (c *Client) startTest(cmd *models.CommandMessage) {
	slog.Info("Executing startTest command", "run_id", cmd.RunID)

	runDir, err := workspace.CreateRunDirectory(cmd.RunID, c.agentID)
	if err != nil {
		slog.Error("Failed to create workspace", "run_id", cmd.RunID, "error", err)
		c.SendStatus(models.StatusMessage{RunID: cmd.RunID, Status: models.StatusFailed})
		return
	}

	jmxUrl, hasJmx := cmd.DownloadURLs["jmx"]
	if !hasJmx || jmxUrl == "" {
		slog.Error("No JMX download URL provided", "run_id", cmd.RunID)
		c.SendStatus(models.StatusMessage{RunID: cmd.RunID, Status: models.StatusFailed})
		return
	}

	jmxPath := filepath.Join(runDir, "script.jmx")
	if err := workspace.DownloadFile(jmxUrl, jmxPath); err != nil {
		slog.Error("Failed to download JMX script", "run_id", cmd.RunID, "error", err)
		c.SendStatus(models.StatusMessage{RunID: cmd.RunID, Status: models.StatusFailed})
		return
	}

	// Initialize JmeterParams map if nil
	if cmd.JmeterParams == nil {
		cmd.JmeterParams = make(map[string]string)
	}

	// Download any dynamically parsed CSV files/assets
	for key, url := range cmd.DownloadURLs {
		if key == "jmx" || key == "plugins" {
			continue // Handled separately
		}

		// Download the data file into the local workspace
		fileName := key + ".csv" // Assuming CSV by default for variables
		filePath := filepath.Join(runDir, fileName)

		if err := workspace.DownloadFile(url, filePath); err != nil {
			slog.Error("Failed to download referenced JMX variable asset", "key", key, "url", url, "error", err)
			c.SendStatus(models.StatusMessage{RunID: cmd.RunID, Status: models.StatusFailed})
			return
		}

		// Inject absolute path back into JMeter parameters so it can resolve ${__P(key)}
		absFilePath, absErr := filepath.Abs(filePath)
		if absErr != nil {
			slog.Error("Failed to get absolute path for CSV", "key", key, "error", absErr)
			c.SendStatus(models.StatusMessage{RunID: cmd.RunID, Status: models.StatusFailed})
			return
		}
		cmd.JmeterParams[key] = absFilePath
		slog.Info("Injected variable mapped file", "key", key, "path", absFilePath)
	}

	// Keep the injector call for compatibility, but the script remains the source of truth.
	slog.Info("Validating script-provided BackendListener configuration", "run_id", cmd.RunID)
	if injectErr := xmlparser.InjectBackendListener(jmxPath); injectErr != nil {
		slog.Warn("BackendListener validation failed, continuing with original script", "run_id", cmd.RunID, "error", injectErr)
	}

	// Derive the HTTP upload URL from the WebSocket URL, handling both ws:// and wss://
	httpURL := c.hubURL
	if parsedURL, parseErr := url.Parse(c.hubURL); parseErr == nil {
		if parsedURL.Scheme == "wss" {
			parsedURL.Scheme = "https"
		} else {
			parsedURL.Scheme = "http"
		}
		parsedURL.Path = ""
		parsedURL.RawQuery = ""
		httpURL = parsedURL.String()
	} else {
		// Fallback: simple string replace for ws://
		httpURL = strings.Replace(c.hubURL, "ws://", "http://", 1)
		if idx := strings.Index(httpURL, "/ws"); idx != -1 {
			httpURL = httpURL[:idx]
		}
	}
	uploadURL := httpURL + "/api/results/upload/" + cmd.RunID + "?agentId=" + c.agentID

	// Set up callbacks
	onLog := func(runID string, logLine string) {
		msg := models.LogMessage{
			RunID:   runID,
			AgentID: c.agentID,
			LogLine: logLine,
		}
		c.SendLog(msg)
	}

	onStatus := func(runID string, err error) {
		status := models.StatusCompleted
		if err != nil {
			status = models.StatusFailed
		}

		msg := models.StatusMessage{
			RunID:   runID,
			AgentID: c.agentID,
			Status:  status,
		}
		c.SendStatus(msg)
	}

	err = executor.Execute(cmd.RunID, c.agentID, jmxPath, cmd.RunMode, cmd.JmeterParams, uploadURL, onLog, onStatus)
	if err != nil {
		slog.Error("Failed to initiate execution", "run_id", cmd.RunID, "error", err)
		c.SendStatus(models.StatusMessage{
			RunID:  cmd.RunID,
			Status: models.StatusFailed,
		})
	}
}

func (c *Client) stopTest(cmd *models.CommandMessage) {
	slog.Info("Executing stopTest command", "run_id", cmd.RunID)
	err := executor.StopTest(cmd.RunID)
	if err != nil {
		slog.Error("Failed to stop test", "run_id", cmd.RunID, "error", err)
	}
}

func (c *Client) SendLog(logMsg models.LogMessage) error {
	return c.sendJSON(logMsg)
}

func (c *Client) SendStatus(statusMsg models.StatusMessage) error {
	return c.sendJSON(statusMsg)
}

func (c *Client) sendJSON(v interface{}) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.conn == nil {
		return websocket.ErrCloseSent
	}
	return c.conn.WriteJSON(v)
}

func (c *Client) Close() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.conn != nil {
		c.conn.Close()
		c.conn = nil
	}
	select {
	case <-c.done:
	default:
		close(c.done)
	}
}
