package api

import (
	"archive/zip"
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"jmeter-hub/database"
	"jmeter-hub/models"
	"jmeter-hub/net"
	"jmeter-hub/parser"

	"github.com/gin-gonic/gin"
)

// SetupRouter initializes the gin server and maps all the API endpoints
func SetupRouter(hub *net.Hub) *gin.Engine {
	r := gin.Default()

	// Ensure our workspace directories exist dynamically
	os.MkdirAll("./master_workspace", 0755)
	os.MkdirAll("./results", 0755)

	// API Groups
	api := r.Group("/api")
	{
		api.POST("/upload/script", UploadScriptHandler(hub))
		api.POST("/parse-jmx", ParseJMXHandler)
		api.POST("/results/upload/:runId", ResultReceiverHandler)
		// api.POST("/v2/write/:runId", InfluxMetricsReceiver(hub))
		// api.POST("/v2/write", InfluxMetricsReceiver(hub)) // Fallback for backwards compatibility
		api.GET("/run/active", GetActiveRunHandler)
		api.GET("/history", GetHistoryHandler)
		api.GET("/agents", func(c *gin.Context) {
			agents := hub.GetAgentIDs()
			if agents == nil {
				agents = []string{}
			}
			c.JSON(http.StatusOK, gin.H{"agents": agents})
		})
	}

	// Serve the static generated JMeter HTML reports.
	// For example, if HTML is generated to ./results/123/report, it becomes accessible at /reports/123/report
	r.StaticFS("/reports", http.Dir("./results"))

	// Serve the uploaded JMX and CSV files for Agents to download
	r.StaticFS("/master_workspace", http.Dir("./master_workspace"))

	// WebSockets
	r.GET("/ws", func(c *gin.Context) {
		net.ServeWs(hub, c.Writer, c.Request)
	})

	return r
}

// UploadScriptHandler accepts a multipart form upload (a .jmx file and optionally .csv files),
// parses the configurations to split/share CSVs dynamically, saves them to a local
// ./master_workspace/ directory, and dispatches start commands over WebSockets to targeted agents.
func UploadScriptHandler(hub *net.Hub) gin.HandlerFunc {
	return func(c *gin.Context) {
		err := c.Request.ParseMultipartForm(32 << 20) // 32MB max memory
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Could not parse multipart form: " + err.Error()})
			return
		}

		form, err := c.MultipartForm()
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Could not get multipart form: " + err.Error()})
			return
		}

		// Parse the config JSON
		configStr := c.PostForm("config")
		if configStr == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Missing 'config' field in form data"})
			return
		}

		var config struct {
			Mode          string            `json:"mode"`
			Agents        []string          `json:"agents"`
			CSVStrategies map[string]string `json:"csvStrategies"`
			HasCSVHeader  bool              `json:"hasCsvHeader"`
		}
		if err := json.Unmarshal([]byte(configStr), &config); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to parse config JSON: " + err.Error()})
			return
		}

		if len(config.Agents) == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "No agents specified in config"})
			return
		}

		// 1. Extract and Save the Main JMX Script
		files := form.File["file"] // Assuming form-data key "file" is the JMX
		if len(files) == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "No .jmx file found in request under 'file' key"})
			return
		}

		jmxFile := files[0]
		primaryScriptName := filepath.Base(jmxFile.Filename)
		runID := fmt.Sprintf("run-%d", time.Now().UnixNano())

		// Create a unique output directory in master_workspace for this run
		runWorkspace := filepath.Join(".", "master_workspace", runID)
		os.MkdirAll(runWorkspace, 0755)

		dstJmx := filepath.Join(runWorkspace, primaryScriptName)
		if err := c.SaveUploadedFile(jmxFile, dstJmx); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save JMX file: " + err.Error()})
			return
		}
		jmxDownloadURL := fmt.Sprintf("http://%s/master_workspace/%s/%s", c.Request.Host, runID, primaryScriptName)

		// 2. Parse the strictly required CSVs from the JMX
		requiredCSVs, err := parser.ParseRequiredCSVs(dstJmx)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to parse JMX script: " + err.Error()})
			return
		}

		// 3. Process required CSVs based on config strategy
		// agentFiles maps AgentID -> map[FormKey]DownloadURL
		agentFiles := make(map[string]map[string]string)
		for _, agent := range config.Agents {
			agentFiles[agent] = make(map[string]string)
		}

		for _, csvName := range requiredCSVs {
			csvFormFiles := form.File[csvName]
			if len(csvFormFiles) == 0 {
				c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Required CSV file missing in form: %s", csvName)})
				return
			}

			csvFile := csvFormFiles[0]
			strategy := config.CSVStrategies[csvName]

			if config.Mode == "distributed" && strategy == "split" {
				// We need to split this CSV across the agents
				tempCsvPath := filepath.Join(runWorkspace, "temp_"+csvFile.Filename)
				if err := c.SaveUploadedFile(csvFile, tempCsvPath); err != nil {
					c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save temp CSV: " + err.Error()})
					return
				}

				chunks, err := parser.SplitCSV(tempCsvPath, len(config.Agents), config.HasCSVHeader)
				if err != nil {
					c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to split CSV: " + err.Error()})
					return
				}

				// Assign each chunk uniquely to the respective agent
				for i, agent := range config.Agents {
					chunkFileName := fmt.Sprintf("%s_%s", agent, csvFile.Filename)
					chunkFilePath := filepath.Join(runWorkspace, chunkFileName)
					if err := parser.SaveCSVChunk(chunks[i], chunkFilePath); err != nil {
						c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save split chunk: " + err.Error()})
						return
					}
					agentFiles[agent][csvName] = fmt.Sprintf("http://%s/master_workspace/%s/%s", c.Request.Host, runID, chunkFileName)
				}

				// Clean up the temp file
				os.Remove(tempCsvPath)
			} else {
				// "share" strategy or "single" machine mode
				// Save it once and give everyone the same download URL
				sharedFilePath := filepath.Join(runWorkspace, csvFile.Filename)
				if err := c.SaveUploadedFile(csvFile, sharedFilePath); err != nil {
					c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save shared CSV: " + err.Error()})
					return
				}

				sharedURL := fmt.Sprintf("http://%s/master_workspace/%s/%s", c.Request.Host, runID, csvFile.Filename)
				for _, agent := range config.Agents {
					agentFiles[agent][csvName] = sharedURL
				}
			}
		}

		// 4. Update Database
		runReq := database.TestRun{
			ID:         runID,
			ScriptName: primaryScriptName,
			Status:     "running",
			StartTime:  time.Now(),
		}
		if err := database.InsertRun(runReq); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to record run in database: " + err.Error()})
			return
		}

		for _, agentID := range config.Agents {
			if err := database.AddRunAgent(runID, agentID, "running"); err != nil {
				slog.Error("Failed to record agent for run", "run_id", runID, "agent_id", agentID, "error", err)
			}
		}

		// 5. Build and Dispatch the Start Command via Hub
		for _, agentID := range config.Agents {
			// Construct downloadURLs: strictly JMX plus any dynamically named CSVs mapped correctly
			downloadUrlsMap := make(map[string]string)
			downloadUrlsMap["jmx"] = jmxDownloadURL

			// Inject specific chunks/shared CSVs needed for this agent
			for key, url := range agentFiles[agentID] {
				downloadUrlsMap[key] = url
			}

			// Use the typed CommandMessage so run_mode and jmeter_params are forwarded to agents
			payload := models.CommandMessage{
				Action:       models.ActionStart,
				RunID:        runID,
				RunMode:      config.Mode,
				DownloadURLs: downloadUrlsMap,
			}

			messageBytes, err := json.Marshal(payload)
			if err != nil {
				slog.Error("Failed to marshal start command", "agent_id", agentID, "error", err)
				continue
			}
			hub.SendCommand(agentID, messageBytes)
		}

		// Return success to the UI (the UI should no longer be sending the start socket message)
		c.JSON(http.StatusOK, gin.H{
			"message": "Test execution started successfully",
			"runId":   runID,
		})
	}
}

// ParseJMXHandler accepts a single JMX file upload, parses it using the parser utility,
// and returns the strictly required CSV variable names.
func ParseJMXHandler(c *gin.Context) {
	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "JMX file is required under 'file' form key"})
		return
	}

	// Ensure an identifiable temp path is used that we have permissions on.
	// We'll just put it in the master_workspace temporarily.
	os.MkdirAll("./master_workspace/temp", 0755)
	tempFilePath := filepath.Join(".", "master_workspace", "temp", fmt.Sprintf("upload-%d.jmx", time.Now().UnixNano()))

	// Save upload to temp file
	if err := c.SaveUploadedFile(file, tempFilePath); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save file: " + err.Error()})
		return
	}
	defer os.Remove(tempFilePath)

	// Parse it
	requiredCSVs, err := parser.ParseRequiredCSVs(tempFilePath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to parse JMX file: " + err.Error()})
		return
	}

	// If nil, return an empty array instead of null
	if requiredCSVs == nil {
		requiredCSVs = []string{}
	}

	c.JSON(http.StatusOK, gin.H{
		"required_csvs": requiredCSVs,
	})
}

// ResultReceiverHandler an endpoint where the remote Agent can POST the zipped .jtl
// results after a test finishes. Unzip these into a ./results/{runID}/ folder.
func ResultReceiverHandler(c *gin.Context) {
	runID := c.Param("runId")
	if runID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "runId parameter is required"})
		return
	}

	agentID := c.Query("agentId")
	if agentID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "agentId query parameter is required"})
		return
	}

	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Zip file is required under 'file' form key"})
		return
	}

	// Ensure the specific result folder exists for this agent
	resultDir := filepath.Join(".", "results", runID, agentID)
	os.MkdirAll(resultDir, 0755)

	zipDst := filepath.Join(resultDir, file.Filename)
	if err := c.SaveUploadedFile(file, zipDst); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save zip: " + err.Error()})
		return
	}

	// Unzip the received file
	if err := unzip(zipDst, resultDir); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to unzip results: " + err.Error()})
		return
	}

	// Remove the zip safely to conserve space if desired
	os.Remove(zipDst)

	// Update the individual agent's status
	// Ensure zip path is registered, separating physical file arrival from logical 'completion'
	_ = database.UpdateRunAgentStatus(runID, agentID, "completed", &resultDir)

	// If all agents' zip files have physically arrived and unzipped, update the overarching Run status
	if database.AreAllAgentsZipped(runID) {
		endTime := time.Now()
		// Just logging path mapping. The actual HTML log generation typically occurs around here.
		logPath := fmt.Sprintf("/reports/%s", runID)

		// Merge final JTLs here
		finalJTLPath := filepath.Join(".", "results", runID, "results.jtl")
		mergedErr := mergeJTLs(runID, finalJTLPath)
		if mergedErr != nil {
			slog.Error("Failed to merge JTL files", "run_id", runID, "error", mergedErr)
		} else {
			slog.Info("Successfully merged agent JTLs into final result", "run_id", runID, "path", finalJTLPath)
		}

		_ = database.UpdateRunStatus(runID, "completed", &endTime, &logPath)
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Results received and extracted successfully",
		"runId":   runID,
	})
}

// GetActiveRunHandler Checks if there's currently a running test and returns the run ID if so
func GetActiveRunHandler(c *gin.Context) {
	run, err := database.GetActiveRun()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to query active run: " + err.Error()})
		return
	}

	if run == nil {
		c.JSON(http.StatusOK, gin.H{
			"active": false,
			"runId":  nil,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"active": true,
		"runId":  run.ID,
	})
}

// GetHistoryHandler Queries the SQLite database and returns the test execution history as JSON
func GetHistoryHandler(c *gin.Context) {
	limit := 100 // Fetching top 100 for now.

	runs, err := database.GetHistory(limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve history: " + err.Error()})
		return
	}

	// Return an empty array instead of null if no logic
	if runs == nil {
		runs = []database.TestRun{}
	}

	c.JSON(http.StatusOK, gin.H{
		"data": runs,
	})
}

// unzip utility safely extracts the zip file from src to dest
func unzip(src string, dest string) error {
	r, err := zip.OpenReader(src)
	if err != nil {
		return err
	}
	defer r.Close()

	for _, f := range r.File {
		// Prevent ZipSlip vulnerability
		fpath := filepath.Join(dest, f.Name)
		if !strings.HasPrefix(fpath, filepath.Clean(dest)+string(os.PathSeparator)) {
			return fmt.Errorf("%s: illegal file path", fpath)
		}

		if f.FileInfo().IsDir() {
			os.MkdirAll(fpath, os.ModePerm)
			continue
		} // else if it's a file

		if err = os.MkdirAll(filepath.Dir(fpath), os.ModePerm); err != nil {
			return err
		}

		outFile, err := os.OpenFile(fpath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
		if err != nil {
			return err
		}

		rc, err := f.Open()
		if err != nil {
			outFile.Close()
			return err
		}

		// Limit decompressed size to 500MB to prevent zip bomb attacks
		const maxDecompressedSize = 500 * 1024 * 1024
		_, err = io.Copy(outFile, io.LimitReader(rc, maxDecompressedSize))

		// Ensure file closes even within the loop explicitly
		outFile.Close()
		rc.Close()

		if err != nil {
			return err
		}
	}
	return nil
}

// mergeJTLs finds all results.jtl files in the agent subdirectories of a run,
// extracts the CSV header from the first one, and appends the data from all of them
// into a single master results.jtl file.
func mergeJTLs(runID string, destPath string) error {
	runDir := filepath.Join(".", "results", runID)
	entries, err := os.ReadDir(runDir)
	if err != nil {
		return err
	}

	outFile, err := os.Create(destPath)
	if err != nil {
		return err
	}
	defer outFile.Close()

	headerWritten := false

	for _, entry := range entries {
		if !entry.IsDir() {
			continue // skip files in the root run directory (like the zip or final jtl)
		}

		agentJTLPath := filepath.Join(runDir, entry.Name(), "results.jtl")
		if _, err := os.Stat(agentJTLPath); os.IsNotExist(err) {
			continue
		}

		agentFile, err := os.Open(agentJTLPath)
		if err != nil {
			slog.Error("Failed to open agent JTL for merging", "path", agentJTLPath, "error", err)
			continue
		}

		scanner := bufio.NewScanner(agentFile)
		isFirstLine := true
		for scanner.Scan() {
			line := scanner.Text()
			if isFirstLine {
				isFirstLine = false
				if !headerWritten {
					if _, werr := outFile.WriteString(line + "\n"); werr != nil {
						slog.Error("Failed to write JTL header", "run_id", runID, "error", werr)
					}
					headerWritten = true
				}
				continue // Skip the header for subsequent files
			}
			if _, werr := outFile.WriteString(line + "\n"); werr != nil {
				slog.Error("Failed to write JTL line", "run_id", runID, "error", werr)
			}
		}

		agentFile.Close()
	}

	return nil
}
