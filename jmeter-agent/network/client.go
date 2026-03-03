package network

import (
	"encoding/json"
	"jmeter-agent/executor"
	"jmeter-agent/models"
	"jmeter-agent/workspace"
	"log/slog"
	"math"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

type Client struct {
	hubURL string
	conn   *websocket.Conn
	mu     sync.Mutex
	done   chan struct{}
}

func NewClient(hubURL string) *Client {
	return &Client{
		hubURL: hubURL,
		done:   make(chan struct{}),
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
			slog.Warn("Unknown command action", "action", cmd.Action)
		}
	}
}

func (c *Client) startTest(cmd *models.CommandMessage) {
	slog.Info("Executing startTest command", "run_id", cmd.RunID)

	runDir, err := workspace.CreateRunDirectory(cmd.RunID)
	if err != nil {
		slog.Error("Failed to create workspace", "run_id", cmd.RunID, "error", err)
		c.SendStatus(models.StatusMessage{RunID: cmd.RunID, Status: models.StatusFailed})
		return
	}

	jmxPath := filepath.Join(runDir, "script.jmx")
	if cmd.DownloadURLs.JMX != "" {
		if err := workspace.DownloadFile(cmd.DownloadURLs.JMX, jmxPath); err != nil {
			slog.Error("Failed to download JMX script", "run_id", cmd.RunID, "error", err)
			c.SendStatus(models.StatusMessage{RunID: cmd.RunID, Status: models.StatusFailed})
			return
		}
	} else {
		slog.Error("No JMX download URL provided", "run_id", cmd.RunID)
		c.SendStatus(models.StatusMessage{RunID: cmd.RunID, Status: models.StatusFailed})
		return
	}

	// Change ws://hostname:port/ws to http://hostname:port/api/results/upload/runID
	httpURL := strings.Replace(c.hubURL, "ws://", "http://", 1)
	if idx := strings.Index(httpURL, "/ws"); idx != -1 {
		httpURL = httpURL[:idx]
	}
	uploadURL := httpURL + "/api/results/upload/" + cmd.RunID

	// Set up callbacks
	onLog := func(runID string, logLine string) {
		msg := models.LogMessage{
			RunID:   runID,
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
			RunID:  runID,
			Status: status,
		}
		c.SendStatus(msg)
	}

	err = executor.Execute(cmd.RunID, jmxPath, cmd.JmeterParams, uploadURL, onLog, onStatus)
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
