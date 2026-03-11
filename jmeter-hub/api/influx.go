package api

import (
	"bufio"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"jmeter-hub/net"

	"github.com/gin-gonic/gin"
)

// InfluxMetricsReceiver acts as a fake InfluxDB endpoint for JMeter's
// Backend Listener, parsing the line-protocol metrics and broadcasting
// them to the web UI via WebSocket.
func InfluxMetricsReceiver(hub *net.Hub) gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.Method != http.MethodPost {
			c.AbortWithStatusJSON(http.StatusMethodNotAllowed, gin.H{"error": "Method not allowed"})
			return
		}

		// JMeter sends line-protocol data in the body
		body, err := io.ReadAll(c.Request.Body)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "Failed to read body"})
			return
		}
		defer c.Request.Body.Close()

		// Parse the InfluxDB Line Protocol data
		scanner := bufio.NewScanner(strings.NewReader(string(body)))
		for scanner.Scan() {
			line := scanner.Text()

			// Example line: jmeter.all.a.count,application=my_performance_test,statut=all,transaction=all value=100 1678886400000000000
			if strings.TrimSpace(line) == "" {
				continue
			}

			// LOG THE RAW LINE TO THE TERMINAL FOR DEBUGGING
			slog.Info("Received raw metric from JMeter", "length", len(line), "line", line)

			// Broadcast it via WebSocket to the UI
			// We format it as a special metric message so the UI knows how to handle it
			payload := struct {
				Type    string `json:"type"`
				LogLine string `json:"log_line"` // For backward compatibility with things expecting log_line
				Data    string `json:"data"`     // The raw InfluxDB line
			}{
				Type:    "metric",
				LogLine: "METRIC: " + line,
				Data:    line,
			}

			payloadBytes, err := json.Marshal(payload)
			if err == nil {
				// Send to the hub broadcast channel
				hub.Broadcast <- payloadBytes
			} else {
				slog.Error("Failed to marshal metric payload", "error", err)
			}
		}

		if err := scanner.Err(); err != nil {
			slog.Error("Error scanning InfluxDB body", "error", err)
		}

		// Tell JMeter we received it successfully (HTTP 204 No Content is standard for InfluxDB)
		c.Status(http.StatusNoContent)
	}
}
