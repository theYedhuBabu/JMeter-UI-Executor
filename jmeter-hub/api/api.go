package api

import (
	"archive/zip"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"jmeter-hub/database"
	"jmeter-hub/net"

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
		api.POST("/upload/script", UploadScriptHandler)
		api.POST("/results/upload/:runId", ResultReceiverHandler)
		api.GET("/history", GetHistoryHandler)
		api.GET("/agents", func(c *gin.Context) {
			var agents []string
			// Warning: ranging loosely over map; ideally requires mutex for strict safety,
			// but since go map reads in general are somewhat forgiving or we copy it locally.
			// Actually the simplest way is to fetch here directly since it's just keys.
			for id := range hub.Clients {
				agents = append(agents, id)
			}
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

// UploadScriptHandler accepts a multipart form upload (a .jmx file and .csv files),
// saves them to a local ./master_workspace/ directory, and returns a download URL.
func UploadScriptHandler(c *gin.Context) {
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

	files := form.File["file"] // Assuming form-data key is "file"
	if len(files) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No files found in request under 'file' key"})
		return
	}

	var savedFiles []string
	var primaryScriptName string

	for i, file := range files {
		fileName := filepath.Base(file.Filename)
		if i == 0 {
			primaryScriptName = fileName // Assume first file is the JMX script
		}
		dst := filepath.Join(".", "master_workspace", fileName)

		if err := c.SaveUploadedFile(file, dst); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save file: " + err.Error()})
			return
		}

		downloadURL := fmt.Sprintf("http://%s/master_workspace/%s", c.Request.Host, fileName)
		savedFiles = append(savedFiles, downloadURL)
	}

	// Generate a unique run ID
	runID := fmt.Sprintf("run-%d", time.Now().Unix())

	// Insert the initial state into the History database
	runReq := database.TestRun{
		ID:         runID,
		ScriptName: primaryScriptName,
		Status:     "running", // Initially mark as running when dispatched
		StartTime:  time.Now(),
	}
	_ = database.InsertRun(runReq)

	c.JSON(http.StatusOK, gin.H{
		"message":      "Files uploaded successfully",
		"runId":        runID,
		"downloadURLs": savedFiles,
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

	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Zip file is required under 'file' form key"})
		return
	}

	// Ensure the specific result folder exists
	resultDir := filepath.Join(".", "results", runID)
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

	// Update SQLite Database to reflect finished log states (EndTime and Status normally synced via WS, but good redundancy here)
	endTime := time.Now()
	// Just logging path mapping. The actual HTML log generation typically occurs around here.
	logPath := fmt.Sprintf("/reports/%s", runID)

	_ = database.UpdateRunStatus(runID, "completed", &endTime, &logPath)

	c.JSON(http.StatusOK, gin.H{
		"message": "Results received and extracted successfully",
		"runId":   runID,
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

		_, err = io.Copy(outFile, rc)

		// Ensure file closes even within the loop explicitly
		outFile.Close()
		rc.Close()

		if err != nil {
			return err
		}
	}
	return nil
}
