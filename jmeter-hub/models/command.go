package models

// CommandAction represents actions sent to agents from the Hub
type CommandAction string

const (
	ActionStart CommandAction = "start"
	ActionStop  CommandAction = "stop"
	ActionPing  CommandAction = "ping"
)

// CommandMessage is the message dispatched to an Agent over WebSocket
type CommandMessage struct {
	Action       CommandAction     `json:"action"`
	RunID        string            `json:"run_id"`
	RunMode      string            `json:"run_mode"`
	DownloadURLs map[string]string `json:"download_urls,omitempty"`
	JmeterParams map[string]string `json:"jmeter_params,omitempty"`
}
