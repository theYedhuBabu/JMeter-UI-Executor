package net

import (
	"encoding/json"
	"fmt"
	"jmeter-hub/database"
	"log/slog"
	"sync"
	"time"
)

// DirectMessage is used by SendCommand to route a message to a specific agent
// through the Hub's safe event loop, avoiding direct map access from HTTP goroutines.
type DirectMessage struct {
	AgentID string
	Payload []byte
}

// Hub maintains the set of active clients (agents) and broadcasts messages to them.
type Hub struct {
	// Registered clients map where key is the Agent ID.
	// This map is ONLY accessed inside the Run() goroutine — never from outside.
	Clients map[string]*Client

	// Inbound messages from the clients.
	Broadcast chan []byte

	// Register requests from the clients.
	Register chan *Client

	// Unregister requests from clients.
	Unregister chan *Client

	// DirectSend routes a targeted message to a single agent safely through Run().
	DirectSend chan DirectMessage

	// clientsMu protects read-only access to Clients from outside Run() (e.g. HTTP handlers).
	clientsMu sync.RWMutex
}

// NewHub initializes a new Hub instance
func NewHub() *Hub {
	return &Hub{
		Broadcast:  make(chan []byte),
		Register:   make(chan *Client),
		Unregister: make(chan *Client),
		DirectSend: make(chan DirectMessage, 32),
		Clients:    make(map[string]*Client),
	}
}

// Run executes a blocking event loop managing concurrent connection state safely
func (h *Hub) Run() {
	for {
		select {
		case client := <-h.Register:
			h.clientsMu.Lock()
			h.Clients[client.AgentID] = client
			h.clientsMu.Unlock()
			slog.Info("Agent registered successfully in Hub", "agent_id", client.AgentID)

		case client := <-h.Unregister:
			h.clientsMu.Lock()
			if existingClient, ok := h.Clients[client.AgentID]; ok {
				if existingClient == client {
					delete(h.Clients, client.AgentID)
					close(client.Send)
					slog.Info("Agent unregistered from Hub", "agent_id", client.AgentID)
				} else {
					// Stale reconnect: a newer client replaced this one — only close the stale channel
					close(client.Send)
					slog.Debug("Ignoring unregister for stale client", "agent_id", client.AgentID)
				}
			}
			h.clientsMu.Unlock()

		case dm := <-h.DirectSend:
			// Safe targeted send — routed through the event loop so Clients map access is single-threaded
			h.clientsMu.RLock()
			targetClient, exists := h.Clients[dm.AgentID]
			h.clientsMu.RUnlock()
			if !exists {
				slog.Warn("Agent not found for direct send", "agent_id", dm.AgentID)
				continue
			}
			select {
			case targetClient.Send <- dm.Payload:
			default:
				h.clientsMu.Lock()
				close(targetClient.Send)
				delete(h.Clients, dm.AgentID)
				h.clientsMu.Unlock()
			}

		case message := <-h.Broadcast:
			var payload struct {
				TargetAgentID string `json:"targetAgentId"`
				LogLine       string `json:"log_line"`
				Status        string `json:"status"`
				RunID         string `json:"run_id"`
				AgentID       string `json:"agent_id"`
			}

			if err := json.Unmarshal(message, &payload); err == nil {
				if payload.Status != "" && payload.RunID != "" {
					// Agent state transition update (e.g., failed)
					// We only process 'failed' via WebSocket to avoid race conditions with HTTP Zip Uploads.
					// A 'completed' status must be explicitly recorded by the '/api/results/upload' endpoint after full file receipt.
					if payload.Status == "failed" {
						_ = database.UpdateRunAgentStatus(payload.RunID, payload.AgentID, payload.Status, nil)
						slog.Info("Agent reported failure via WS", "run_id", payload.RunID, "agent_id", payload.AgentID, "status", payload.Status)

						// In a failure state, still check if this was the last running agent
						if database.AreAllAgentsFinished(payload.RunID) {
							endTime := time.Now()
							logPath := fmt.Sprintf("/reports/%s", payload.RunID)
							_ = database.UpdateRunStatus(payload.RunID, "completed", &endTime, &logPath)
							slog.Info("All agents finished (some failed) via WS, run is closed", "run_id", payload.RunID)
						}
					} else {
						slog.Debug("Agent reported completion via WS, awaiting Zip upload HTTP hit to mark DB", "run_id", payload.RunID, "agent_id", payload.AgentID)
					}

					// Route this status message to the UI as well
					h.sendToUI(message)
				} else if payload.TargetAgentID != "" {
					// Targeted message from UI to a specific Agent
					h.clientsMu.RLock()
					targetClient, ok := h.Clients[payload.TargetAgentID]
					h.clientsMu.RUnlock()
					if ok {
						select {
						case targetClient.Send <- message:
						default:
							h.clientsMu.Lock()
							close(targetClient.Send)
							delete(h.Clients, payload.TargetAgentID)
							h.clientsMu.Unlock()
						}
					} else {
						slog.Warn("Target agent not found for message", "agent_id", payload.TargetAgentID)
					}
				} else if payload.LogLine != "" {
					// It's a log message from an Agent, route it to the web-ui
					h.sendToUI(message)
				} else {
					// Broadcast to all (fallback for unrecognized messages)
					h.broadcastAll(message)
				}
			} else {
				// Fallback broadcast if JSON fails to parse
				h.broadcastAll(message)
			}
		}
	}
}

// sendToUI routes a message to the web-ui client if connected.
func (h *Hub) sendToUI(message []byte) {
	h.clientsMu.RLock()
	uiClient, ok := h.Clients["web-ui"]
	h.clientsMu.RUnlock()
	if !ok {
		return
	}
	select {
	case uiClient.Send <- message:
	default:
		h.clientsMu.Lock()
		close(uiClient.Send)
		delete(h.Clients, "web-ui")
		h.clientsMu.Unlock()
	}
}

// broadcastAll sends a message to every connected client.
func (h *Hub) broadcastAll(message []byte) {
	h.clientsMu.RLock()
	clients := make(map[string]*Client, len(h.Clients))
	for id, c := range h.Clients {
		clients[id] = c
	}
	h.clientsMu.RUnlock()

	for agentID, client := range clients {
		select {
		case client.Send <- message:
		default:
			h.clientsMu.Lock()
			close(client.Send)
			delete(h.Clients, agentID)
			h.clientsMu.Unlock()
		}
	}
}

// SendCommand is used to push specific json message structs to an individual agent.
// It is safe to call from any goroutine (HTTP handlers, etc.) as it routes through DirectSend.
func (h *Hub) SendCommand(agentID string, message []byte) error {
	h.DirectSend <- DirectMessage{AgentID: agentID, Payload: message}
	slog.Info("Command queued for agent via WebSocket", "agent_id", agentID)
	return nil
}

// GetAgentIDs returns a snapshot of all connected agent IDs (excluding web-ui).
// Safe to call from any goroutine.
func (h *Hub) GetAgentIDs() []string {
	h.clientsMu.RLock()
	defer h.clientsMu.RUnlock()
	var agents []string
	for id := range h.Clients {
		if id != "web-ui" {
			agents = append(agents, id)
		}
	}
	return agents
}
