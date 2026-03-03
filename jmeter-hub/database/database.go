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

	// Configuration for robust concurrency
	DB.SetMaxOpenConns(25)
	DB.SetMaxIdleConns(25)
	DB.SetConnMaxLifetime(5 * time.Minute)

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

	return runs, nil
}
