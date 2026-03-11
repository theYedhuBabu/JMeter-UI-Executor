package database

import (
	"database/sql"
	"fmt"
	"log/slog"
	"time"

	_ "github.com/mattn/go-sqlite3" // Import go-sqlite3 driver
)

var DB *sql.DB

// TestRun represents a single JMeter execution run
type TestRun struct {
	ID         string
	ScriptName string
	Status     string
	StartTime  time.Time
	EndTime    *time.Time // pointer because it can be null
	LogPath    *string
}

// Agent represents a registered JMeter agent
type Agent struct {
	ID        string
	IPAddress string
	Status    string
	LastSeen  time.Time
}

// InitializeDB opens the SQLite database and creates tables if they don't exist
func InitializeDB(dbPath string) error {
	var err error

	// Open connection pooling
	DB, err = sql.Open("sqlite3", dbPath)
	if err != nil {
		slog.Error("Failed to open database", "path", dbPath, "error", err)
		return err
	}

	// Ping to verify connection
	if err = DB.Ping(); err != nil {
		slog.Error("Failed to ping database", "error", err)
		return err
	}

	// SQLite supports only one writer at a time.
	// Setting MaxOpenConns > 1 causes SQLITE_BUSY errors under concurrent writes.
	DB.SetMaxOpenConns(1)
	DB.SetMaxIdleConns(1)
	DB.SetConnMaxLifetime(5 * time.Minute)

	// Enable WAL mode for better read concurrency with a single writer
	if _, err := DB.Exec("PRAGMA journal_mode=WAL;"); err != nil {
		slog.Warn("Failed to enable WAL mode", "error", err)
	}

	slog.Info("Database connection established", "path", dbPath)

	return createTables()
}

func createTables() error {
	createRunsTable := `
	CREATE TABLE IF NOT EXISTS TestRuns (
		ID TEXT PRIMARY KEY,
		ScriptName TEXT NOT NULL,
		Status TEXT NOT NULL,
		StartTime DATETIME NOT NULL,
		EndTime DATETIME,
		LogPath TEXT
	);`

	createRunAgentsTable := `
	CREATE TABLE IF NOT EXISTS RunAgents (
		RunID TEXT NOT NULL,
		AgentID TEXT NOT NULL,
		Status TEXT NOT NULL,
		ZipPath TEXT,
		PRIMARY KEY (RunID, AgentID)
	);`

	createAgentsTable := `
	CREATE TABLE IF NOT EXISTS Agents (
		ID TEXT PRIMARY KEY,
		IPAddress TEXT NOT NULL,
		Status TEXT NOT NULL,
		LastSeen DATETIME NOT NULL
	);`

	if _, err := DB.Exec(createRunsTable); err != nil {
		slog.Error("Failed to create TestRuns table", "error", err)
		return fmt.Errorf("create TestRuns table: %w", err)
	}

	if _, err := DB.Exec(createRunAgentsTable); err != nil {
		slog.Error("Failed to create RunAgents table", "error", err)
		return fmt.Errorf("create RunAgents table: %w", err)
	}

	if _, err := DB.Exec(createAgentsTable); err != nil {
		slog.Error("Failed to create Agents table", "error", err)
		return fmt.Errorf("create Agents table: %w", err)
	}

	slog.Info("Database tables initialized successfully")
	return nil
}

// InsertRun inserts a new test run into the database
func InsertRun(run TestRun) error {
	query := `INSERT INTO TestRuns (ID, ScriptName, Status, StartTime, EndTime, LogPath)
			  VALUES (?, ?, ?, ?, ?, ?)`

	_, err := DB.Exec(query, run.ID, run.ScriptName, run.Status, run.StartTime, run.EndTime, run.LogPath)
	if err != nil {
		slog.Error("Failed to insert run", "run_id", run.ID, "error", err)
		return err
	}
	return nil
}

// UpdateRunStatus updates the status, end time, and log path of a test run
func UpdateRunStatus(id string, status string, endTime *time.Time, logPath *string) error {
	query := `UPDATE TestRuns SET Status = ?, EndTime = ?, LogPath = ? WHERE ID = ?`

	result, err := DB.Exec(query, status, endTime, logPath, id)
	if err != nil {
		slog.Error("Failed to update run status", "run_id", id, "error", err)
		return err
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		slog.Warn("UpdateRunStatus affected 0 rows, run ID might not exist", "run_id", id)
	}
	return nil
}

// RegisterAgent inserts or updates an agent's details and last seen timestamp
func RegisterAgent(agent Agent) error {
	// Upsert query for SQLite (using INSERT ... ON CONFLICT REPLACE does similarly,
	// but standard SQLite UPSERT syntax is cleaner for 3.24.0+)
	query := `
	INSERT INTO Agents (ID, IPAddress, Status, LastSeen)
	VALUES (?, ?, ?, ?)
	ON CONFLICT(ID) DO UPDATE SET 
		IPAddress=excluded.IPAddress, 
		Status=excluded.Status, 
		LastSeen=excluded.LastSeen;`

	_, err := DB.Exec(query, agent.ID, agent.IPAddress, agent.Status, agent.LastSeen)
	if err != nil {
		slog.Error("Failed to register agent", "agent_id", agent.ID, "error", err)
		return err
	}
	return nil
}

// AddRunAgent inserts a specific agent assigned to a test run
func AddRunAgent(runID string, agentID string, status string) error {
	query := `INSERT INTO RunAgents (RunID, AgentID, Status) VALUES (?, ?, ?)`
	_, err := DB.Exec(query, runID, agentID, status)
	if err != nil {
		slog.Error("Failed to insert run agent", "run_id", runID, "agent_id", agentID, "error", err)
		return err
	}
	return nil
}

// UpdateRunAgentStatus updates the status and downloaded zip path of a specific agent's execution
func UpdateRunAgentStatus(runID string, agentID string, status string, zipPath *string) error {
	query := `UPDATE RunAgents SET Status = ?, ZipPath = ? WHERE RunID = ? AND AgentID = ?`
	_, err := DB.Exec(query, status, zipPath, runID, agentID)
	if err != nil {
		slog.Error("Failed to update run agent status", "run_id", runID, "agent_id", agentID, "error", err)
		return err
	}
	return nil
}

// AreAllAgentsFinished checks if all agents assigned to a run have logically completed or failed
func AreAllAgentsFinished(runID string) bool {
	query := `SELECT COUNT(*) FROM RunAgents WHERE RunID = ? AND Status NOT IN ('completed', 'failed')`
	var count int
	err := DB.QueryRow(query, runID).Scan(&count)
	if err != nil {
		slog.Error("Failed to check if all agents finished", "run_id", runID, "error", err)
		return false
	}
	return count == 0
}

// AreAllAgentsZipped checks if all logically successful agents have actually uploaded their result zip bundles
func AreAllAgentsZipped(runID string) bool {
	// A run is fully zipped if there are zero agents that are "completed" but lack a ZipPath.
	// Failed agents or still-running agents don't count towards blocking the zip merge,
	// though AreAllAgentsFinished should theoretically guard still-running agents anyway.
	query := `SELECT COUNT(*) FROM RunAgents WHERE RunID = ? AND Status = 'completed' AND ZipPath IS NULL`
	var count int
	err := DB.QueryRow(query, runID).Scan(&count)
	if err != nil {
		slog.Error("Failed to check if all agents are zipped", "run_id", runID, "error", err)
		return false
	}
	return count == 0
}

// GetHistory retrieves the most recent N test runs
func GetHistory(limit int) ([]TestRun, error) {
	query := `SELECT ID, ScriptName, Status, StartTime, EndTime, LogPath 
			  FROM TestRuns ORDER BY StartTime DESC LIMIT ?`

	rows, err := DB.Query(query, limit)
	if err != nil {
		slog.Error("Failed to query run history", "error", err)
		return nil, err
	}
	defer rows.Close()

	var runs []TestRun
	for rows.Next() {
		var run TestRun
		err := rows.Scan(
			&run.ID,
			&run.ScriptName,
			&run.Status,
			&run.StartTime,
			&run.EndTime,
			&run.LogPath,
		)
		if err != nil {
			slog.Error("Failed to scan run row", "error", err)
			return nil, err
		}
		runs = append(runs, run)
	}

	if err := rows.Err(); err != nil {
		slog.Error("Error iterating run history rows", "error", err)
		return nil, err
	}

	return runs, nil
}

// GetActiveRun retrieves the first test run with 'running' status
func GetActiveRun() (*TestRun, error) {
	query := `SELECT ID, ScriptName, Status, StartTime, EndTime, LogPath 
			  FROM TestRuns WHERE Status = 'running' LIMIT 1`

	var run TestRun
	err := DB.QueryRow(query).Scan(
		&run.ID,
		&run.ScriptName,
		&run.Status,
		&run.StartTime,
		&run.EndTime,
		&run.LogPath,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		slog.Error("Failed to query active run", "error", err)
		return nil, err
	}

	return &run, nil
}

// ResetZombieRuns resets the status of any runs still marked as 'running' to 'failed'
// This is typically called on Hub startup to clear out orphaned runs from previous crashes.
// It also resets the corresponding RunAgents rows so AreAllAgentsFinished returns correct results.
func ResetZombieRuns() error {
	// First reset the parent run records
	result, err := DB.Exec(`UPDATE TestRuns SET Status = 'failed' WHERE Status = 'running'`)
	if err != nil {
		slog.Error("Failed to reset zombie runs", "error", err)
		return err
	}
	rowsAffected, _ := result.RowsAffected()
	if rowsAffected > 0 {
		slog.Info("Reset zombie test runs", "count", rowsAffected)
	}

	// Also reset the per-agent rows so AreAllAgentsFinished works correctly for these runs
	_, err = DB.Exec(`
		UPDATE RunAgents
		SET Status = 'failed'
		WHERE Status = 'running'
		  AND RunID IN (SELECT ID FROM TestRuns WHERE Status = 'failed')`)
	if err != nil {
		slog.Error("Failed to reset zombie run agents", "error", err)
		return err
	}

	return nil
}
