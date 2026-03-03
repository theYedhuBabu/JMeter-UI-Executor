package models

import "time"

// CommandAction represents actions received from the Hub
type CommandAction string

const (
	ActionStart CommandAction = "start"
	ActionStop  CommandAction = "stop"
	ActionPing  CommandAction = "ping"
)

// AgentStatus represents the state of the agent or specific run
type AgentStatus string

const (
	StatusRunning   AgentStatus = "running"
	StatusStopped   AgentStatus = "stopped"
	StatusFailed    AgentStatus = "failed"
	StatusCompleted AgentStatus = "completed"
)

// DownloadURLs contains URLs to download JMX and plugin files
type DownloadURLs struct {
	JMX     string `json:"jmx"`
	Plugins string `json:"plugins,omitempty"`
}

// CommandMessage is received from the Hub
type CommandMessage struct {
	Action       CommandAction     `json:"action"`
	RunID        string            `json:"run_id"`
	DownloadURLs DownloadURLs      `json:"download_urls,omitempty"`
	JmeterParams map[string]string `json:"jmeter_params,omitempty"`
}

// StatusMessage is sent to the Hub
type StatusMessage struct {
	RunID   string      `json:"run_id"`
	Status  AgentStatus `json:"status"`
	Message string      `json:"message,omitempty"`
}

// LogMessage is sent to the Hub to stream execution logs
type LogMessage struct {
	RunID     string    `json:"run_id"`
	LogLine   string    `json:"log_line"`
	Timestamp time.Time `json:"timestamp"`
}
