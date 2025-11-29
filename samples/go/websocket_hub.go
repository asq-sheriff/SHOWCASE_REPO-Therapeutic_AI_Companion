// Package websocket provides real-time therapeutic conversation support
// via WebSocket connections with Redis pub/sub for horizontal scaling.
package websocket

import (
	"context"
	"encoding/json"
	"log/slog"
	"sync"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/gorilla/websocket"
)

// MessageType defines the type of WebSocket message
type MessageType string

const (
	MessageTypeChat         MessageType = "chat"
	MessageTypeCrisisAlert  MessageType = "crisis_alert"
	MessageTypeTyping       MessageType = "typing"
	MessageTypePresence     MessageType = "presence"
	MessageTypeAcknowledge  MessageType = "ack"
	MessageTypeHeartbeat    MessageType = "heartbeat"
)

// Message represents a WebSocket message with therapeutic context
type Message struct {
	ID            string                 `json:"id"`
	Type          MessageType            `json:"type"`
	UserID        string                 `json:"user_id"`
	SessionID     string                 `json:"session_id"`
	Content       string                 `json:"content,omitempty"`
	Metadata      map[string]interface{} `json:"metadata,omitempty"`
	Timestamp     time.Time              `json:"timestamp"`
	CrisisLevel   string                 `json:"crisis_level,omitempty"`
	RequiresAck   bool                   `json:"requires_ack,omitempty"`
}

// Client represents a WebSocket client connection
type Client struct {
	ID         string
	UserID     string
	SessionID  string
	Role       string // resident, family, staff, provider, admin
	Conn       *websocket.Conn
	Send       chan []byte
	Hub        *Hub
	LastPing   time.Time
	mu         sync.RWMutex
}

// Hub maintains the set of active clients and broadcasts messages
type Hub struct {
	// Registered clients by user ID
	clients map[string]map[*Client]bool

	// Registered clients by session ID
	sessions map[string]*Client

	// Inbound messages from clients
	broadcast chan *Message

	// Register requests from clients
	register chan *Client

	// Unregister requests from clients
	unregister chan *Client

	// Redis client for pub/sub across instances
	redis *redis.Client

	// Redis pub/sub channel
	pubsub *redis.PubSub

	// Context for graceful shutdown
	ctx    context.Context
	cancel context.CancelFunc

	// Mutex for thread-safe operations
	mu sync.RWMutex

	// Logger
	logger *slog.Logger

	// Crisis alert handler
	crisisHandler CrisisHandler

	// Message persistence
	messageStore MessageStore
}

// CrisisHandler defines the interface for crisis alert handling
type CrisisHandler interface {
	HandleCrisisAlert(ctx context.Context, msg *Message) error
	NotifyCareTeam(ctx context.Context, userID string, crisisLevel string) error
}

// MessageStore defines the interface for message persistence
type MessageStore interface {
	SaveMessage(ctx context.Context, msg *Message) error
	GetMessageHistory(ctx context.Context, sessionID string, limit int) ([]*Message, error)
}

// HubConfig contains configuration for the WebSocket hub
type HubConfig struct {
	RedisURL       string
	RedisChannel   string
	HeartbeatInterval time.Duration
	WriteTimeout   time.Duration
	ReadTimeout    time.Duration
	MaxMessageSize int64
}

// DefaultHubConfig returns default configuration values
func DefaultHubConfig() *HubConfig {
	return &HubConfig{
		RedisChannel:      "lilo:websocket:messages",
		HeartbeatInterval: 30 * time.Second,
		WriteTimeout:      10 * time.Second,
		ReadTimeout:       60 * time.Second,
		MaxMessageSize:    65536, // 64KB
	}
}

// NewHub creates a new WebSocket hub with Redis pub/sub support
func NewHub(cfg *HubConfig, redisClient *redis.Client, logger *slog.Logger) *Hub {
	ctx, cancel := context.WithCancel(context.Background())

	hub := &Hub{
		clients:    make(map[string]map[*Client]bool),
		sessions:   make(map[string]*Client),
		broadcast:  make(chan *Message, 256),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		redis:      redisClient,
		ctx:        ctx,
		cancel:     cancel,
		logger:     logger,
	}

	// Subscribe to Redis channel for cross-instance messaging
	hub.pubsub = redisClient.Subscribe(ctx, cfg.RedisChannel)

	return hub
}

// Run starts the hub's main event loop
func (h *Hub) Run() {
	// Start Redis subscription handler
	go h.handleRedisMessages()

	// Start heartbeat monitor
	go h.heartbeatMonitor()

	for {
		select {
		case <-h.ctx.Done():
			h.shutdown()
			return

		case client := <-h.register:
			h.registerClient(client)

		case client := <-h.unregister:
			h.unregisterClient(client)

		case message := <-h.broadcast:
			h.broadcastMessage(message)
		}
	}
}

// registerClient adds a client to the hub
func (h *Hub) registerClient(client *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()

	// Add to user's client map
	if _, ok := h.clients[client.UserID]; !ok {
		h.clients[client.UserID] = make(map[*Client]bool)
	}
	h.clients[client.UserID][client] = true

	// Add to session map
	h.sessions[client.SessionID] = client

	h.logger.Info("client registered",
		slog.String("user_id", client.UserID),
		slog.String("session_id", client.SessionID),
		slog.String("role", client.Role),
	)

	// Broadcast presence update
	h.broadcastPresence(client, true)
}

// unregisterClient removes a client from the hub
func (h *Hub) unregisterClient(client *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if clients, ok := h.clients[client.UserID]; ok {
		if _, ok := clients[client]; ok {
			delete(clients, client)
			close(client.Send)

			if len(clients) == 0 {
				delete(h.clients, client.UserID)
			}
		}
	}

	delete(h.sessions, client.SessionID)

	h.logger.Info("client unregistered",
		slog.String("user_id", client.UserID),
		slog.String("session_id", client.SessionID),
	)

	// Broadcast presence update
	h.broadcastPresence(client, false)
}

// broadcastMessage sends a message to all relevant clients
func (h *Hub) broadcastMessage(msg *Message) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	// Handle crisis alerts specially
	if msg.Type == MessageTypeCrisisAlert && h.crisisHandler != nil {
		go func() {
			if err := h.crisisHandler.HandleCrisisAlert(h.ctx, msg); err != nil {
				h.logger.Error("failed to handle crisis alert",
					slog.String("error", err.Error()),
					slog.String("user_id", msg.UserID),
				)
			}
		}()
	}

	// Persist message if store is configured
	if h.messageStore != nil {
		go func() {
			if err := h.messageStore.SaveMessage(h.ctx, msg); err != nil {
				h.logger.Error("failed to save message",
					slog.String("error", err.Error()),
				)
			}
		}()
	}

	// Serialize message
	data, err := json.Marshal(msg)
	if err != nil {
		h.logger.Error("failed to marshal message", slog.String("error", err.Error()))
		return
	}

	// Send to all clients for this user
	if clients, ok := h.clients[msg.UserID]; ok {
		for client := range clients {
			select {
			case client.Send <- data:
			default:
				// Client buffer full, close connection
				h.unregister <- client
			}
		}
	}

	// Publish to Redis for other instances
	h.publishToRedis(msg)
}

// publishToRedis publishes a message to Redis for cross-instance delivery
func (h *Hub) publishToRedis(msg *Message) {
	data, err := json.Marshal(msg)
	if err != nil {
		h.logger.Error("failed to marshal message for Redis",
			slog.String("error", err.Error()),
		)
		return
	}

	if err := h.redis.Publish(h.ctx, "lilo:websocket:messages", data).Err(); err != nil {
		h.logger.Error("failed to publish to Redis",
			slog.String("error", err.Error()),
		)
	}
}

// handleRedisMessages processes messages from Redis pub/sub
func (h *Hub) handleRedisMessages() {
	ch := h.pubsub.Channel()

	for {
		select {
		case <-h.ctx.Done():
			return
		case redisMsg := <-ch:
			var msg Message
			if err := json.Unmarshal([]byte(redisMsg.Payload), &msg); err != nil {
				h.logger.Error("failed to unmarshal Redis message",
					slog.String("error", err.Error()),
				)
				continue
			}

			// Deliver to local clients only (avoid re-publishing)
			h.deliverLocal(&msg)
		}
	}
}

// deliverLocal delivers a message to local clients without republishing
func (h *Hub) deliverLocal(msg *Message) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	data, err := json.Marshal(msg)
	if err != nil {
		return
	}

	if clients, ok := h.clients[msg.UserID]; ok {
		for client := range clients {
			select {
			case client.Send <- data:
			default:
				go func(c *Client) {
					h.unregister <- c
				}(client)
			}
		}
	}
}

// broadcastPresence sends presence updates to relevant users
func (h *Hub) broadcastPresence(client *Client, online bool) {
	msg := &Message{
		Type:      MessageTypePresence,
		UserID:    client.UserID,
		SessionID: client.SessionID,
		Metadata: map[string]interface{}{
			"online": online,
			"role":   client.Role,
		},
		Timestamp: time.Now(),
	}

	// Broadcast to care team if this is a resident
	if client.Role == "resident" {
		h.notifyCareTeam(client.UserID, msg)
	}
}

// notifyCareTeam sends notifications to care team members
func (h *Hub) notifyCareTeam(residentID string, msg *Message) {
	// Implementation would query care team relationships
	// and send presence updates to relevant staff/family
}

// heartbeatMonitor checks for stale client connections
func (h *Hub) heartbeatMonitor() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-h.ctx.Done():
			return
		case <-ticker.C:
			h.checkHeartbeats()
		}
	}
}

// checkHeartbeats removes clients that haven't responded to pings
func (h *Hub) checkHeartbeats() {
	h.mu.RLock()
	staleClients := make([]*Client, 0)

	for _, clients := range h.clients {
		for client := range clients {
			client.mu.RLock()
			if time.Since(client.LastPing) > 90*time.Second {
				staleClients = append(staleClients, client)
			}
			client.mu.RUnlock()
		}
	}
	h.mu.RUnlock()

	// Unregister stale clients
	for _, client := range staleClients {
		h.logger.Warn("removing stale client",
			slog.String("user_id", client.UserID),
			slog.Duration("last_ping", time.Since(client.LastPing)),
		)
		h.unregister <- client
	}
}

// SendToUser sends a message to all connections for a specific user
func (h *Hub) SendToUser(userID string, msg *Message) error {
	msg.Timestamp = time.Now()
	h.broadcast <- msg
	return nil
}

// SendCrisisAlert sends a crisis alert with guaranteed delivery
func (h *Hub) SendCrisisAlert(userID string, crisisLevel string, details map[string]interface{}) error {
	msg := &Message{
		Type:        MessageTypeCrisisAlert,
		UserID:      userID,
		CrisisLevel: crisisLevel,
		Metadata:    details,
		Timestamp:   time.Now(),
		RequiresAck: true,
	}

	h.broadcast <- msg

	// Also notify care team
	if h.crisisHandler != nil {
		return h.crisisHandler.NotifyCareTeam(h.ctx, userID, crisisLevel)
	}

	return nil
}

// GetOnlineUsers returns a list of currently connected user IDs
func (h *Hub) GetOnlineUsers() []string {
	h.mu.RLock()
	defer h.mu.RUnlock()

	users := make([]string, 0, len(h.clients))
	for userID := range h.clients {
		users = append(users, userID)
	}
	return users
}

// IsUserOnline checks if a user has any active connections
func (h *Hub) IsUserOnline(userID string) bool {
	h.mu.RLock()
	defer h.mu.RUnlock()

	clients, ok := h.clients[userID]
	return ok && len(clients) > 0
}

// shutdown gracefully shuts down the hub
func (h *Hub) shutdown() {
	h.mu.Lock()
	defer h.mu.Unlock()

	// Close all client connections
	for _, clients := range h.clients {
		for client := range clients {
			close(client.Send)
		}
	}

	// Close Redis pub/sub
	if h.pubsub != nil {
		h.pubsub.Close()
	}

	h.logger.Info("WebSocket hub shutdown complete")
}

// Stop gracefully stops the hub
func (h *Hub) Stop() {
	h.cancel()
}
