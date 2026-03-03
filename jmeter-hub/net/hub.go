package net

import (
	"encoding/json"
	"log/slog"
)

// Hub maintains the set of active clients (agents) and broadcasts messages to them.
type Hub struct {
	// Registered clients map where key is the Agent ID
	Clients map[string]*Client

	// Inbound messages from the clients.
	Broadcast chan []byte

	// Register requests from the clients.
	Register chan *Client

	// Unregister requests from clients.
	Unregister chan *Client
}

// NewHub initializes a new Hub instance
func NewHub() *Hub {
	return &Hub{
		Broadcast:  make(chan []byte),
		Register:   make(chan *Client),
		Unregister: make(chan *Client),
		Clients:    make(map[string]*Client),
	}
}

// Run executes a blocking event loop managing concurrent connection state safely
func (h *Hub) Run() {
	for {
		select {
		case client := <-h.Register:
			h.Clients[client.AgentID] = client
			slog.Info("Agent registered successfully in Hub", "agent_id", client.AgentID)

		case client := <-h.Unregister:
			if _, ok := h.Clients[client.AgentID]; ok {
				delete(h.Clients, client.AgentID)
				close(client.Send)
				slog.Info("Agent unregistered from Hub", "agent_id", client.AgentID)
			}

		case message := <-h.Broadcast:
			// Parse the message to see if it has a target agentId (from UI) or is a log (from Agent)
			var payload struct {
				AgentID string `json:"agentId"`
				LogLine string `json:"log_line"`
			}

			if err := json.Unmarshal(message, &payload); err == nil {
				if payload.AgentID != "" {
					// Targeted message from UI to a specific Agent
					if targetClient, ok := h.Clients[payload.AgentID]; ok {
						select {
						case targetClient.Send <- message:
						default:
							close(targetClient.Send)
							delete(h.Clients, payload.AgentID)
						}
					} else {
						slog.Warn("Target agent not found for message", "agent_id", payload.AgentID)
					}
				} else if payload.LogLine != "" {
					// It's a log message from an Agent, route it to the web-ui
					if uiClient, ok := h.Clients["web-ui"]; ok {
						select {
						case uiClient.Send <- message:
						default:
							close(uiClient.Send)
							delete(h.Clients, "web-ui")
						}
					}
				} else {
					// Broadcast to everyone (fallback)
					for _, client := range h.Clients {
						select {
						case client.Send <- message:
						default:
							close(client.Send)
							delete(h.Clients, client.AgentID)
						}
					}
				}
			} else {
				// Fallback broadcast if JSON fails to parse
				for _, client := range h.Clients {
					select {
					case client.Send <- message:
					default:
						close(client.Send)
						delete(h.Clients, client.AgentID)
					}
				}
			}
		}
	}
}

// SendCommand is used to push specific json message structs to an individual agent
func (h *Hub) SendCommand(agentID string, message []byte) error {
	client, exists := h.Clients[agentID]
	if !exists {
		slog.Warn("Agent not found in active hub connections", "agent_id", agentID)
		return nil
	}

	client.Send <- message
	slog.Info("Command sent to agent via WebSocket", "agent_id", agentID)
	return nil
}
