package net

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // WARNING: Allow any origin for demo purposes
	},
}

// Client is a middleman between the websocket connection and the hub.
type Client struct {
	Hub     *Hub
	AgentID string
	Conn    *websocket.Conn
	Send    chan []byte
}

func (c *Client) readPump() {
	defer func() {
		c.Hub.Unregister <- c
		c.Conn.Close()
	}()

	// Limit incoming message size to 10MB to prevent memory spikes from rogue clients
	c.Conn.SetReadLimit(10 * 1024 * 1024)

	for {
		_, message, err := c.Conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				slog.Error("WebSocket unexpected close error", "error", err)
			}
			break
		}

		h := c.Hub
		h.Broadcast <- message
	}
}

func (c *Client) writePump() {
	ticker := time.NewTicker(54 * time.Second)
	defer func() {
		ticker.Stop()
		c.Conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.Send:
			if !ok {
				c.Conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := c.Conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			w.Write(message)

			if err := w.Close(); err != nil {
				return
			}
		case <-ticker.C:
			if err := c.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// ServeWs handles websocket requests from the peer.
func ServeWs(hub *Hub, w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		slog.Error("Failed to upgrade websocket", "error", err)
		return
	}

	agentID := r.URL.Query().Get("agentId")
	if agentID == "" {
		slog.Warn("WebSocket received connection without agentId")
		conn.Close()
		return
	}

	client := &Client{
		Hub:     hub,
		AgentID: agentID,
		Conn:    conn,
		Send:    make(chan []byte, 256),
	}

	client.Hub.Register <- client

	// Allow collection of memory referenced by the caller by doing all work in
	// new goroutines.
	go client.writePump()
	go client.readPump()
}
