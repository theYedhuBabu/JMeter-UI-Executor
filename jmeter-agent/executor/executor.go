package executor

import (
	"bufio"
	"fmt"
	"io"
	"jmeter-agent/cleanup"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"
)

// LogCallback is a function signature for handling real-time log lines
type LogCallback func(runID string, logLine string)

// StatusCallback is a function signature for handling completion or failure
type StatusCallback func(runID string, err error)

var (
	runningCmds   = make(map[string]*exec.Cmd)
	runningCmdsMu sync.Mutex
)

// Execute runs the JMeter CLI asynchronously
func Execute(runID string, agentID string, jmxPath string, runMode string, jmeterParams map[string]string, uploadURL string, onLog LogCallback, onStatus StatusCallback) error {
	slog.Info("Preparing to execute JMeter", "run_id", runID, "jmx", jmxPath)

	runDir := filepath.Dir(jmxPath)
	resultsJTL := filepath.Join(runDir, "results.jtl")

	absJmxPath, err := filepath.Abs(jmxPath)
	if err != nil {
		slog.Error("Failed to get absolute path for JMX", "run_id", runID, "error", err)
		if onStatus != nil {
			onStatus(runID, err)
		}
		return err
	}

	absResultsJTL, err := filepath.Abs(resultsJTL)
	if err != nil {
		slog.Error("Failed to get absolute path for JTL", "run_id", runID, "error", err)
		if onStatus != nil {
			onStatus(runID, err)
		}
		return err
	}

	absLogPath, err := filepath.Abs(filepath.Join(runDir, "jmeter.log"))
	if err != nil {
		slog.Error("Failed to get absolute path for Log", "run_id", runID, "error", err)
		if onStatus != nil {
			onStatus(runID, err)
		}
		return err
	}

	// Configure InfluxDB Backend Listener dynamically
	influxURL := uploadURL // We will override this carefully below
	if strings.Contains(uploadURL, "/api/results/upload") {
		influxURL = strings.Replace(uploadURL, "/api/results/upload", "/api/v2/write", 1) + "?db=jmeter"
	}

	// Construct base command
	args := []string{
		"-n", // non-GUI mode
		"-t", absJmxPath,
		"-l", absResultsJTL,
		"-j", absLogPath,
	}

	for key, val := range jmeterParams {
		args = append(args, fmt.Sprintf("-J%s=%s", key, val))
	}

	// Make the agentID available as a JMeter property
	args = append(args, fmt.Sprintf("-JagentId=%s", agentID))

	// Inject the influxDB parameters automatically if not provided
	if _, ok := jmeterParams["influxdbUrl"]; !ok {
		args = append(args,
			fmt.Sprintf("-JinfluxdbUrl=%s", influxURL),
			"-JinfluxdbMetricsSender=org.apache.jmeter.visualizers.backend.influxdb.HttpMetricsSender",
			fmt.Sprintf("-Japplication=jmeter_%s", agentID),
			"-JsummaryOnly=false",
		)
	}

	// Prioritize `jmeter` from system PATH if available
	jmeterExec := "jmeter"
	if _, err := exec.LookPath(jmeterExec); err != nil {
		// Fallback to JMETER_HOME environment variable if not in system PATH
		jmeterHome := os.Getenv("JMETER_HOME")
		if jmeterHome != "" {
			localPath := filepath.Join(jmeterHome, "bin", "jmeter")
			if _, statErr := os.Stat(localPath); statErr == nil {
				jmeterExec = localPath
			} else {
				errMsg := fmt.Errorf("JMeter not found in PATH and JMETER_HOME/bin/jmeter is invalid: %v", statErr)
				slog.Error("Environment error", "run_id", runID, "error", errMsg)
				if onLog != nil {
					onLog(runID, fmt.Sprintf("FATAL: %v", errMsg))
				}
				if onStatus != nil {
					onStatus(runID, errMsg)
				}
				return errMsg
			}
		} else {
			errMsg := fmt.Errorf("JMeter not found in PATH and JMETER_HOME environment variable is not set")
			slog.Error("Environment error", "run_id", runID, "error", errMsg)
			if onLog != nil {
				onLog(runID, fmt.Sprintf("FATAL: %v", errMsg))
			}
			if onStatus != nil {
				onStatus(runID, errMsg)
			}
			return errMsg
		}
	}

	cmd := exec.Command(jmeterExec, args...)
	cmd.Dir = runDir

	// Configure Process Group ID for safe termination (Linux/Unix)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		slog.Error("Failed to get stdout pipe", "run_id", runID, "error", err)
		return err
	}

	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		slog.Error("Failed to get stderr pipe", "run_id", runID, "error", err)
		return err
	}

	if err := cmd.Start(); err != nil {
		slog.Error("Failed to start JMeter process", "run_id", runID, "error", err, "executable_used", jmeterExec)
		if onLog != nil {
			onLog(runID, fmt.Sprintf("FATAL: Failed to start JMeter process. Executable `%s` could not be run: %v", jmeterExec, err))
		}
		return err
	}

	// Store the command reference
	runningCmdsMu.Lock()
	runningCmds[runID] = cmd
	runningCmdsMu.Unlock()

	slog.Info("JMeter process started successfully", "run_id", runID, "pid", cmd.Process.Pid)
	if onLog != nil {
		onLog(runID, fmt.Sprintf("INFO: Successfully launched JMeter process (PID: %d)", cmd.Process.Pid))
	}

	go func() {
		var wgLog sync.WaitGroup
		wgLog.Add(2)

		// Read from stdout and stderr concurrently
		go func() {
			defer wgLog.Done()
			streamLogs(runID, stdoutPipe, onLog)
		}()
		go func() {
			defer wgLog.Done()
			streamLogs(runID, stderrPipe, onLog)
		}()

		// Wait for logs to finish printing before we call Wait() which closes the pipes.
		wgLog.Wait()

		// Wait for process to exit
		err := cmd.Wait()

		// Remove from map when finished
		runningCmdsMu.Lock()
		delete(runningCmds, runID)
		runningCmdsMu.Unlock()

		if err != nil {
			slog.Error("JMeter process finished with error", "run_id", runID, "error", err)
		} else {
			slog.Info("JMeter process completed successfully", "run_id", runID)
		}

		// Sequential cleanup logic
		zipPath, errZip := cleanup.ArchiveResults(runID, agentID)
		if errZip == nil {
			if uploadURL != "" {
				if errUp := cleanup.UploadResults(zipPath, uploadURL); errUp != nil {
					slog.Error("Failed to upload results", "run_id", runID, "error", errUp)
				}
			} else {
				slog.Warn("No uploadURL provided, skipping results upload", "run_id", runID)
			}
		} else {
			slog.Error("Failed to archive results", "run_id", runID, "error", errZip)
		}

		if errWipe := cleanup.WipeWorkspace(runID, agentID); errWipe != nil {
			slog.Error("Failed to wipe workspace", "run_id", runID, "error", errWipe)
		}

		if onStatus != nil {
			onStatus(runID, err)
		}
	}()

	return nil
}

func streamLogs(runID string, pipe io.Reader, onLog LogCallback) {
	scanner := bufio.NewScanner(pipe)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == "" {
			continue
		}
		if onLog != nil {
			onLog(runID, line)
		}
	}
	if err := scanner.Err(); err != nil {
		slog.Error("Error scanning logs", "run_id", runID, "error", err)
	}
}

// StopTest sends a SIGTERM to the process group associated with the runID,
// giving JMeter a chance to flush results. Falls back to SIGKILL after 10 seconds.
func StopTest(runID string) error {
	runningCmdsMu.Lock()
	cmd, exists := runningCmds[runID]
	runningCmdsMu.Unlock()

	if !exists || cmd == nil || cmd.Process == nil {
		slog.Warn("Attempted to stop run, but no active process found", "run_id", runID)
		return fmt.Errorf("no active process found for runID: %s", runID)
	}

	pgid, err := syscall.Getpgid(cmd.Process.Pid)
	if err != nil {
		slog.Error("Failed to get process group ID", "run_id", runID, "pid", cmd.Process.Pid, "error", err)
		return err
	}

	slog.Info("Sending SIGTERM to process group", "run_id", runID, "pgid", pgid)
	if err := syscall.Kill(-pgid, syscall.SIGTERM); err != nil {
		slog.Error("Failed to send SIGTERM to process group", "run_id", runID, "pgid", pgid, "error", err)
		return err
	}

	// Give JMeter up to 10 seconds to flush results before forcefully killing it
	done := make(chan struct{})
	go func() {
		cmd.Wait() // nolint: this is a best-effort wait
		close(done)
	}()

	select {
	case <-done:
		slog.Info("Process exited cleanly after SIGTERM", "run_id", runID)
	case <-time.After(10 * time.Second):
		slog.Warn("Process did not exit after SIGTERM, sending SIGKILL", "run_id", runID, "pgid", pgid)
		if err := syscall.Kill(-pgid, syscall.SIGKILL); err != nil {
			slog.Error("Failed to send SIGKILL to process group", "run_id", runID, "pgid", pgid, "error", err)
			return err
		}
	}

	return nil
}
