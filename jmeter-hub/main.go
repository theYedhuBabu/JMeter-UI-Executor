package main

import (
	"embed"
	"io"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"strings"

	"github.com/gin-gonic/gin"

	"jmeter-hub/api"
	"jmeter-hub/database"
	"jmeter-hub/net"
)

//go:embed ui/dist/*
var staticFiles embed.FS

func main() {
	// 1. Initialize Logging
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)
	slog.Info("Starting JMeter Central Hub...")

	// 2. Initialize Database
	database.InitializeDB("./jmeter_hub.db")

	// 3. Initialize and Start the WebSocket Hub
	wsHub := net.NewHub()
	go wsHub.Run()

	// 4. Set up Routing (API, WS, Uploads)
	router := api.SetupRouter(wsHub)

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
					contentType := "text/plain"
					if strings.HasSuffix(path, ".css") {
						contentType = "text/css"
					} else if strings.HasSuffix(path, ".js") {
						contentType = "application/javascript"
					} else if strings.HasSuffix(path, ".svg") {
						contentType = "image/svg+xml"
					} else if strings.HasSuffix(path, ".html") {
						contentType = "text/html; charset=utf-8"
					} else if strings.HasSuffix(path, ".png") {
						contentType = "image/png"
					} else if strings.HasSuffix(path, ".ico") {
						contentType = "image/x-icon"
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
	port := ":8080"
	slog.Info("Hub listening", "port", port)
	if err := router.Run(port); err != nil {
		slog.Error("Server failed", "error", err)
		os.Exit(1)
	}
}
