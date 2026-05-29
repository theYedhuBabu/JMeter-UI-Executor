package main

import (
	"embed"
	"io"
	"io/fs"
	"log/slog"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	stdnet "net"

	"github.com/gin-gonic/gin"

	"jmeter-hub/api"
	"jmeter-hub/database"
	"jmeter-hub/net"
	"jmeter-hub/services"
)

//go:embed ui/dist/*
var staticFiles embed.FS

func main() {
	// 1. Initialize Logging
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)
	slog.Info("Starting JMeter Central Hub...")

	// 2. Initialize Database and Reset Stuck State
	database.InitializeDB("./jmeter_hub.db")
	database.ResetZombieRuns()

	// 3. Initialize and Start the WebSocket Hub
	wsHub := net.NewHub()
	go wsHub.Run()

	conn, _ := stdnet.Dial("udp", "8.8.8.8:80")
	lanIP := conn.LocalAddr().(*stdnet.UDPAddr).IP.String()
	conn.Close()
	port := ":8080"
	// 4. Set up Routing (API, WS, Uploads)
	router := api.SetupRouter(wsHub, lanIP+port)

	// Start Kafka metrics pipeline
	services.StartKafkaMetricsPipeline(
		wsHub,
		[]string{"localhost:29092"},
		"jmeter_metrics",
	)

	// --- Serve the Embedded React App ---
	// Extract the "ui/dist" directory from the embedded files
	distFS, err := fs.Sub(staticFiles, "ui/dist")
	if err != nil {
		slog.Error("Failed to load embedded static files. Ensure 'ui/dist' exists.", "error", err)
		os.Exit(1)
	}

	// The gin router handles static directories a bit differently.
	// `api.SetupRouter` returns a `*gin.Engine`. Since we're serving the root path ('/'),
	// and SPA routing (like react-router) normally needs a catch-all route to return index.html,
	// we will map NoRoute to serve index.html, and use gin's StaticFS to serve assets.

	// Serve the static files under a specific route or root.
	// Serving static assets (CSS/JS)
	router.StaticFS("/assets", http.FS(distFS))

	// Catch-all to serve index.html for SPA routing
	router.NoRoute(func(c *gin.Context) {
		path := c.Request.URL.Path

		if path != "/" {
			file, err := distFS.Open(path[1:]) // remove leading slash
			if err == nil {
				defer file.Close()
				fi, _ := file.Stat()
				if !fi.IsDir() {
					content, _ := io.ReadAll(file)
					// Detect MIME type by extension using the standard library
					contentType := mime.TypeByExtension(filepath.Ext(path))
					if contentType == "" {
						contentType = "application/octet-stream"
					}
					c.Data(http.StatusOK, contentType, content)
					return
				}
			}
		}

		// Fallback to serving index.html
		file, err := distFS.Open("index.html")
		if err != nil {
			c.String(http.StatusInternalServerError, "index.html not found")
			return
		}
		defer file.Close()
		content, _ := io.ReadAll(file)
		c.Data(http.StatusOK, "text/html; charset=utf-8", content)
	})

	// 5. Start the Server
	
	slog.Info("Hub listening", "port", port)
	if err := router.Run(port); err != nil {
		slog.Error("Server failed", "error", err)
		os.Exit(1)
	}
}
