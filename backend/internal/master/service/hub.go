package service

import (
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"

	"github.com/Fanc1er/Kate/backend/pkg/errs"
	"github.com/Fanc1er/Kate/backend/pkg/response"
)

// Hub WebSocket 实时事件推送（单租户，全局广播）。
type Hub struct {
	mu       sync.RWMutex
	clients  map[*wsClient]bool
	upgrader websocket.Upgrader
}

type wsClient struct {
	conn *websocket.Conn
	send chan []byte
}

// NewHub 构造 Hub。
func NewHub() *Hub {
	return &Hub{
		clients: make(map[*wsClient]bool),
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool { return true },
		},
	}
}

// ServeWS 升级 WebSocket 连接。握手校验 JWT。
func (h *Hub) ServeWS(tokens *TokenManager) gin.HandlerFunc {
	return func(c *gin.Context) {
		// token 从 query 或 Authorization 头获取。
		token := c.Query("token")
		if token == "" {
			token = strings.TrimSpace(strings.TrimPrefix(c.GetHeader("Authorization"), "Bearer "))
		}
		if token == "" {
			response.FailCode(c, errs.CodeAuthFailed)
			c.Abort()
			return
		}
		if _, err := tokens.Parse(token); err != nil {
			response.Fail(c, errs.FromError(err))
			c.Abort()
			return
		}
		conn, err := h.upgrader.Upgrade(c.Writer, c.Request, nil)
		if err != nil {
			return
		}
		client := &wsClient{conn: conn, send: make(chan []byte, 64)}
		h.register(client)
		go h.writePump(client)
		go h.readPump(client)
	}
}

// Broadcast 向全部连接推送消息帧。
func (h *Hub) Broadcast(msg map[string]any) {
	data, err := json.Marshal(msg)
	if err != nil {
		return
	}
	h.mu.RLock()
	defer h.mu.RUnlock()
	for cl := range h.clients {
		select {
		case cl.send <- data:
		default:
		}
	}
}

func (h *Hub) register(cl *wsClient) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.clients[cl] = true
}

func (h *Hub) unregister(cl *wsClient) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if _, ok := h.clients[cl]; ok {
		delete(h.clients, cl)
		close(cl.send)
	}
}

func (h *Hub) writePump(cl *wsClient) {
	ticker := time.NewTicker(50 * time.Second)
	defer func() {
		ticker.Stop()
		cl.conn.Close()
	}()
	for {
		select {
		case data, ok := <-cl.send:
			if !ok {
				cl.conn.WriteMessage(websocket.CloseMessage, nil)
				return
			}
			if err := cl.conn.WriteMessage(websocket.TextMessage, data); err != nil {
				return
			}
		case <-ticker.C:
			if err := cl.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func (h *Hub) readPump(cl *wsClient) {
	defer h.unregister(cl)
	cl.conn.SetReadLimit(4096)
	cl.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	cl.conn.SetPongHandler(func(string) error {
		cl.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})
	for {
		_, msg, err := cl.conn.ReadMessage()
		if err != nil {
			return
		}
		var frame struct {
			Type string `json:"type"`
		}
		_ = json.Unmarshal(msg, &frame)
		switch frame.Type {
		case "ping":
			_ = cl.conn.WriteJSON(map[string]string{"type": "pong"})
		case "subscribe":
			// 全局订阅，无需额外处理。
		}
	}
}
