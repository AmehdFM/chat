package handlers

import (
	"chat/config"
	"chat/models"
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/gorilla/websocket"
)

var (
	upgrader = websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	}
	connections = make(map[int]*websocket.Conn)
	connMutex   sync.RWMutex
)

type WebSocketHandler struct {
	db *sql.DB
}

func NewWebSocketHandler(db *sql.DB) *WebSocketHandler {
	return &WebSocketHandler{db: db}
}

func (h *WebSocketHandler) HandleStream(c *gin.Context) {
	userID := c.GetInt("user_id")
	username := c.GetString("username")

	h.handleWebSocketConnection(c, userID, username)
}

func (h *WebSocketHandler) HandleStreamWithToken(c *gin.Context) {
	tokenString := c.Query("token")
	if tokenString == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Token requerido en query param"})
		return
	}

	cfg := config.Load()
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		return []byte(cfg.JWTSecret), nil
	})

	if err != nil || !token.Valid {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Token inválido o expirado"})
		return
	}

	var userID int
	var username string
	if claims, ok := token.Claims.(jwt.MapClaims); ok {
		userID = int(claims["user_id"].(float64))
		username = claims["username"].(string)
	} else {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Claims inválidos"})
		return
	}

	h.handleWebSocketConnection(c, userID, username)
}

func (h *WebSocketHandler) handleWebSocketConnection(c *gin.Context, userID int, username string) {
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("❌ Error actualizando a WebSocket: %v", err)
		return
	}

	connMutex.Lock()
	connections[userID] = conn
	connMutex.Unlock()

	log.Printf("✅ Usuario %s (ID: %d) conectado al stream", username, userID)

	welcomeMsg := map[string]interface{}{
		"type":    "connected",
		"message": "Conectado al stream",
		"user_id": userID,
	}
	conn.WriteJSON(welcomeMsg)

	defer func() {
		connMutex.Lock()
		delete(connections, userID)
		connMutex.Unlock()
		conn.Close()
		log.Printf("❌ Usuario %s (ID: %d) desconectado", username, userID)
	}()

	for {
		messageType, _, err := conn.ReadMessage()
		if err != nil {
			log.Printf("⚠️ Error leyendo mensaje de %s: %v", username, err)
			break
		}

		if messageType == websocket.PingMessage {
			conn.WriteMessage(websocket.PongMessage, nil)
		}
	}
}

func notifyNewMessage(userID int, message models.Message) {
	connMutex.RLock()
	conn, exists := connections[userID]
	connMutex.RUnlock()

	if !exists {
		log.Printf("ℹ️ Usuario %d no está conectado, mensaje no enviado por WS", userID)
		return
	}

	payload := map[string]interface{}{
		"type":    "new_message",
		"message": message,
	}

	data, err := json.Marshal(payload)
	if err != nil {
		log.Printf("❌ Error serializando mensaje: %v", err)
		return
	}

	err = conn.WriteMessage(websocket.TextMessage, data)
	if err != nil {
		log.Printf("❌ Error enviando mensaje a usuario %d: %v", userID, err)
		connMutex.Lock()
		delete(connections, userID)
		connMutex.Unlock()
		conn.Close()
	} else {
		log.Printf("✅ Mensaje enviado por WS a usuario %d", userID)
	}
}
